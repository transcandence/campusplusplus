package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

type StayRepository struct {
	pool *pgxpool.Pool
}

func NewStayRepository(pool *pgxpool.Pool) *StayRepository {
	return &StayRepository{pool: pool}
}

const stayColumns = `
	s.id, s.student_id, s.dorm_unit_id, s.start_date, s.end_date, s.status::text,
	s.assigned_by, s.checked_in_by, s.checked_in_at, s.checked_out_by, s.checked_out_at,
	s.cancelled_by, s.cancelled_at, s.cancellation_reason, s.created_at, s.updated_at`

const stayFrom = `
	FROM stays s
	JOIN dorm_units du ON du.id = s.dorm_unit_id
	JOIN floors     f  ON f.id  = du.floor_id
	JOIN buildings  b  ON b.id  = f.building_id
	JOIN residences r  ON r.id  = b.residence_id`

func scanStay(row pgx.Row, withCampus bool) (*model.Stay, int64, error) {
	var (
		st       model.Stay
		status   string
		campusID int64
	)
	dest := []any{
		&st.ID, &st.StudentID, &st.DormUnitID, &st.StartDate, &st.EndDate, &status,
		&st.AssignedBy, &st.CheckedInBy, &st.CheckedInAt, &st.CheckedOutBy, &st.CheckedOutAt,
		&st.CancelledBy, &st.CancelledAt, &st.CancellationReason, &st.CreatedAt, &st.UpdatedAt,
	}
	if withCampus {
		dest = append(dest, &campusID)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, 0, err
	}
	st.Status = model.StayStatus(status)
	return &st, campusID, nil
}

func stayQuery(where string) string {
	cols := stayColumns + ", r.campus_id"
	return "SELECT " + cols + stayFrom + " " + where
}

func (r *StayRepository) List(ctx context.Context, f model.StayFilter) ([]model.Stay, error) {
	var status *string
	if f.Status != nil {
		s := string(*f.Status)
		status = &s
	}

	query := stayQuery(`
	WHERE ($1::text   IS NULL OR s.status::text = $1)
	  AND ($2::bigint IS NULL OR s.student_id   = $2)
	  AND ($3::bigint IS NULL OR s.dorm_unit_id = $3)
	  AND ($4::bigint IS NULL OR r.campus_id    = $4)` + `
	ORDER BY s.id DESC`)

	rows, err := r.pool.Query(ctx, query, status, f.StudentID, f.DormUnitID, f.CampusID)
	if err != nil {
		return nil, fmt.Errorf("list stays: %w", err)
	}
	defer rows.Close()

	out := make([]model.Stay, 0)
	for rows.Next() {
		st, _, err := scanStay(rows, true)
		if err != nil {
			return nil, fmt.Errorf("scan stay: %w", err)
		}
		out = append(out, *st)
	}
	return out, rows.Err()
}

func (r *StayRepository) Get(ctx context.Context, id int64) (*model.Stay, int64, error) {
	st, campusID, err := scanStay(r.pool.QueryRow(ctx, stayQuery("WHERE s.id = $1"), id), true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, model.ErrStayNotFound
		}
		return nil, 0, fmt.Errorf("get stay %d: %w", id, err)
	}
	return st, campusID, nil
}

func (r *StayRepository) GetActiveByStudent(ctx context.Context, studentID int64) (*model.Stay, error) {
	st, _, err := scanStay(
		r.pool.QueryRow(ctx, stayQuery("WHERE s.student_id = $1 AND s.status = 'active' LIMIT 1"), studentID),
		true,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrStayNotFound
		}
		return nil, fmt.Errorf("get active stay: %w", err)
	}
	return st, nil
}

func (r *StayRepository) FindUnitWithPolicy(ctx context.Context, unitID int64) (model.UnitWithPolicy, error) {
	const query = `
		SELECT du.id, du.floor_id, du.unit_number, du.code, du.unit_type::text, du.capacity,
		       r.gender_policy::text, r.campus_id
		FROM dorm_units du
		JOIN floors     f ON f.id = du.floor_id
		JOIN buildings  b ON b.id = f.building_id
		JOIN residences r ON r.id = b.residence_id
		WHERE du.id = $1`

	var u model.UnitWithPolicy
	err := r.pool.QueryRow(ctx, query, unitID).Scan(
		&u.ID, &u.FloorID, &u.UnitNumber, &u.Code, &u.UnitType, &u.Capacity, &u.GenderPolicy, &u.CampusID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.UnitWithPolicy{}, model.ErrUnitNotFound
		}
		return model.UnitWithPolicy{}, fmt.Errorf("find unit %d: %w", unitID, err)
	}
	return u, nil
}

func (r *StayRepository) CreateStay(ctx context.Context, p model.CreateStayParams) (*model.Stay, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const lockUnit = `SELECT unit_type::text, capacity FROM dorm_units WHERE id = $1 FOR UPDATE`
	var (
		unitType string
		capacity int
	)
	if err := tx.QueryRow(ctx, lockUnit, p.DormUnitID).Scan(&unitType, &capacity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUnitNotFound
		}
		return nil, fmt.Errorf("lock unit %d: %w", p.DormUnitID, err)
	}
	if unitType != "room" && unitType != "apart" {
		return nil, model.ErrUnitNotLiving
	}

	const countOverlapping = `
	SELECT count(*) FROM stays
	WHERE dorm_unit_id = $1
	  AND status IN ('planned', 'active')
	  AND start_date < $3      -- existing starts before the new one ends
	  AND end_date   > $2      -- existing ends after the new one starts`

	var overlapping int
	if err := tx.QueryRow(ctx, countOverlapping, p.DormUnitID, p.StartDate, p.EndDate).Scan(&overlapping); err != nil {
		return nil, fmt.Errorf("count overlapping stays: %w", err)
	}
	if overlapping >= capacity {
		return nil, model.ErrUnitFull
	}
	const insert = `
		INSERT INTO stays (student_id, dorm_unit_id, start_date, end_date, status, assigned_by)
		VALUES ($1, $2, $3, $4, 'planned', $5)
		RETURNING id`
	var id int64
	if err := tx.QueryRow(ctx, insert, p.StudentID, p.DormUnitID, p.StartDate, p.EndDate, p.AssignedBy).Scan(&id); err != nil {
		return nil, mapStayWriteError(err)
	}

	stay, _, err := getStayTx(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create stay: %w", err)
	}
	return stay, nil
}

// CheckInStay re-validates the state machine and capacity under locks.
func (r *StayRepository) CheckInStay(ctx context.Context, id, by int64) (*model.Stay, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stay, _, err := getStayTx(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if stay.Status != model.StayStatusPlanned {
		return nil, model.ErrIllegalTransition
	}

	const lockUnit = `SELECT capacity FROM dorm_units WHERE id = $1 FOR UPDATE`
	var capacity int
	if err := tx.QueryRow(ctx, lockUnit, stay.DormUnitID).Scan(&capacity); err != nil {
		return nil, fmt.Errorf("lock unit %d: %w", stay.DormUnitID, err)
	}

	const countActive = `SELECT count(*) FROM stays WHERE dorm_unit_id = $1 AND status = 'active'`
	var active int
	if err := tx.QueryRow(ctx, countActive, stay.DormUnitID).Scan(&active); err != nil {
		return nil, fmt.Errorf("count active stays: %w", err)
	}
	if active >= capacity {
		return nil, model.ErrUnitFull
	}

	const update = `UPDATE stays SET status = 'active', checked_in_by = $2, checked_in_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, update, id, by); err != nil {
		return nil, mapStayWriteError(err)
	}

	updated, _, err := getStayTx(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit check-in: %w", err)
	}
	return updated, nil
}

func (r *StayRepository) CheckOutStay(ctx context.Context, id, by int64) (*model.Stay, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stay, _, err := getStayTx(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if stay.Status != model.StayStatusActive {
		return nil, model.ErrIllegalTransition
	}

	const update = `UPDATE stays SET status = 'ended', checked_out_by = $2, checked_out_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, update, id, by); err != nil {
		return nil, mapStayWriteError(err)
	}

	updated, _, err := getStayTx(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit check-out: %w", err)
	}
	return updated, nil
}

func (r *StayRepository) CancelStay(ctx context.Context, id, by int64, reason *string) (*model.Stay, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stay, _, err := getStayTx(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if stay.Status != model.StayStatusPlanned {
		return nil, model.ErrIllegalTransition
	}

	const update = `
		UPDATE stays
		   SET status = 'cancelled', cancelled_by = $2, cancelled_at = now(), cancellation_reason = $3
		 WHERE id = $1`
	if _, err := tx.Exec(ctx, update, id, by, reason); err != nil {
		return nil, mapStayWriteError(err)
	}

	updated, _, err := getStayTx(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancel: %w", err)
	}
	return updated, nil
}

// getStayTx is the in-transaction twin of Get. It exists so the atomic methods
// above can read the row they are about to change under the same lock.
func getStayTx(ctx context.Context, tx pgx.Tx, id int64, forUpdate bool) (*model.Stay, int64, error) {
	query := stayQuery("WHERE s.id = $1")
	if forUpdate {
		query += " FOR UPDATE OF s"
	}
	st, campusID, err := scanStay(tx.QueryRow(ctx, query, id), true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, model.ErrStayNotFound
		}
		return nil, 0, fmt.Errorf("get stay %d: %w", id, err)
	}
	return st, campusID, nil
}
