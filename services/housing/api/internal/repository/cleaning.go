// Package repository provides PostgreSQL access for the housing service.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

// CleaningRepository owns all cleaning/assignment queries.
type CleaningRepository struct {
	pool *pgxpool.Pool
}

func NewCleaningRepository(pool *pgxpool.Pool) *CleaningRepository {
	return &CleaningRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------



// The scope (operator id / campus id) is a mandatory first argument, never a
// filter. shiftId/status/dormUnitId are AND-ed with it, so passing another
// operator's shiftId can only narrow the result to nothing.



// getTaskForUpdateQuery is the ownership + state read used by start/complete.
// OF t locks only the task row, not the shift. No rows means "not found or not
// yours", which the handler must map to 404 without leaking which one.


// ---------------------------------------------------------------------------
// createCleaningRequest
// ---------------------------------------------------------------------------

// CreateFromActiveStay inserts the request only when the student currently has
// an active stay. The INSERT ... SELECT makes the check and the write atomic:
// no row is written when the SELECT yields nothing.
//
// scheduledDate must be a civil date (UTC midnight is fine); slot is the
// half-day bucket (morning/afternoon); isoYear/isoWeek are computed by the
// service from scheduledDate.
func (r *CleaningRepository) CreateFromActiveStay(
	ctx context.Context,
	studentID int64,
	scheduledDate time.Time,
	slot model.CleaningSlot,
	isoYear, isoWeek int,
) (*model.CleaningRequest, error) {
	const createCleaningRequestQuery = `
		INSERT INTO cleaning_requests (
		    dorm_unit_id, student_id, stay_id, scheduled_date, slot, status, year, iso_week
		)
		SELECT s.dorm_unit_id, s.student_id, s.id, $2::date, $3::cleaning_slot, 'pending', $4, $5
		FROM stays s
		WHERE s.student_id = $1
		  AND s.status = 'active'
		LIMIT 1
		RETURNING id, dorm_unit_id, student_id, stay_id, scheduled_date, slot,
		          status, year, iso_week, created_at, updated_at;
	`

	req, err := scanCleaningRequest(r.pool.QueryRow(ctx, createCleaningRequestQuery,
		studentID, scheduledDate, slot, isoYear, isoWeek,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNoActiveStay
		}
		return nil, fmt.Errorf("create cleaning request: %w", mapCleaningRequestError(err))
	}
	return req, nil
}

// ---------------------------------------------------------------------------
// listMyCleaningRequests
// ---------------------------------------------------------------------------

func (r *CleaningRepository) ListRequestsByStudent(
	ctx context.Context,
	studentID int64,
) ([]model.CleaningRequest, error) {

	const listRequestsByStudentQuery = `
		SELECT id, dorm_unit_id, student_id, stay_id, scheduled_date, slot,
		       status, year, iso_week, created_at, updated_at
		FROM cleaning_requests
		WHERE student_id = $1
		ORDER BY created_at DESC, id DESC;
	`
	rows, err := r.pool.Query(ctx, listRequestsByStudentQuery, studentID)
	if err != nil {
		return nil, fmt.Errorf("list cleaning requests: %w", err)
	}
	defer rows.Close()

	requests := make([]model.CleaningRequest, 0)
	for rows.Next() {
		var req model.CleaningRequest
		if err := rows.Scan(
			&req.ID, &req.DormUnitID, &req.StudentID, &req.StayID,
			&req.ScheduledDate, &req.Slot, &req.Status, &req.Year, &req.ISOWeek,
			&req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cleaning request: %w", err)
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cleaning requests: %w", err)
	}
	return requests, nil
}

// ---------------------------------------------------------------------------
// listMyShifts
// ---------------------------------------------------------------------------

// ListShiftsByOperator returns the shifts of one operator for one civil date.
// The caller resolves "today" in the service timezone; SQL never guesses it.
func (r *CleaningRepository) ListShiftsByOperator(
	ctx context.Context,
	operatorID int64,
	shiftDate time.Time,
) ([]model.CleaningShift, error) {


	const listShiftsByOperatorQuery = `
		SELECT id, operator_id, supervisor_id, campus_id, shift_date,
		       start_time::text, end_time::text, status, created_at, updated_at
		FROM cleaning_shifts
		WHERE operator_id = $1
		  AND shift_date = $2::date
		ORDER BY start_time, id;
	`

	rows, err := r.pool.Query(ctx, listShiftsByOperatorQuery, operatorID, shiftDate)
	if err != nil {
		return nil, fmt.Errorf("list cleaning shifts: %w", err)
	}
	defer rows.Close()

	shifts := make([]model.CleaningShift, 0)
	for rows.Next() {
		var s model.CleaningShift
		if err := rows.Scan(
			&s.ID, &s.OperatorID, &s.SupervisorID, &s.CampusID,
			&s.ShiftDate, &s.StartTime, &s.EndTime, &s.Status,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cleaning shift: %w", err)
		}
		shifts = append(shifts, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cleaning shifts: %w", err)
	}
	return shifts, nil
}

// ---------------------------------------------------------------------------
// listCleaningTasks
// ---------------------------------------------------------------------------

// ListTasksForOperator is the operator-scoped list. The operator id is the
// authorization boundary; the filter is applied on top of it.
func (r *CleaningRepository) ListTasksForOperator(
	ctx context.Context,
	operatorID int64,
	filter model.TaskFilter,
) ([]model.CleaningTask, error) {
	const listTasksForOperatorQuery = `
		SELECT t.id, t.shift_id, t.dorm_unit_id, t.cleaning_request_id,
		       t.task_order, t.status, t.completed_at, t.notes,
		       t.created_at, t.updated_at
		FROM cleaning_tasks t
		JOIN cleaning_shifts s ON s.id = t.shift_id
		WHERE s.operator_id = $1
		  AND ($2::bigint IS NULL OR t.shift_id = $2)
		  AND ($3::cleaning_task_status IS NULL OR t.status = $3)
		  AND ($4::bigint IS NULL OR t.dorm_unit_id = $4)
		ORDER BY t.shift_id, t.task_order, t.id;
	`

	return r.listTasks(ctx, listTasksForOperatorQuery,
		operatorID, filter.ShiftID, filter.Status, filter.DormUnitID,
	)
}

// ListTasksForSupervisor is the supervisor-scoped list. The campus id is the
// authorization boundary; the filter is applied on top of it.
func (r *CleaningRepository) ListTasksForSupervisor(
	ctx context.Context,
	campusID int64,
	filter model.TaskFilter,
) ([]model.CleaningTask, error) {
	const listTasksForSupervisorQuery = `
		SELECT t.id, t.shift_id, t.dorm_unit_id, t.cleaning_request_id,
		       t.task_order, t.status, t.completed_at, t.notes,
		       t.created_at, t.updated_at
		FROM cleaning_tasks t
		JOIN cleaning_shifts s ON s.id = t.shift_id
		WHERE s.campus_id = $1
		  AND ($2::bigint IS NULL OR t.shift_id = $2)
		  AND ($3::cleaning_task_status IS NULL OR t.status = $3)
		  AND ($4::bigint IS NULL OR t.dorm_unit_id = $4)
		ORDER BY t.shift_id, t.task_order, t.id;
	`
	return r.listTasks(ctx, listTasksForSupervisorQuery,
		campusID, filter.ShiftID, filter.Status, filter.DormUnitID,
	)
}

func (r *CleaningRepository) listTasks(
	ctx context.Context,
	query string,
	scopeID int64,
	shiftID *int64,
	status *string,
	dormUnitID *int64,
) ([]model.CleaningTask, error) {
	rows, err := r.pool.Query(ctx, query, scopeID, shiftID, status, dormUnitID)
	if err != nil {
		// An invalid status value fails the `$3::cleaning_task_status` cast
		// with 22P02; surface it as ErrInvalidInput (422), not a 500.
		return nil, fmt.Errorf("list cleaning tasks: %w", mapCleaningRequestError(err))
	}
	defer rows.Close()

	tasks := make([]model.CleaningTask, 0)
	for rows.Next() {
		var t model.CleaningTask
		if err := rows.Scan(
			&t.ID, &t.ShiftID, &t.DormUnitID, &t.CleaningRequestID,
			&t.TaskOrder, &t.Status, &t.CompletedAt, &t.Notes,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cleaning task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cleaning tasks: %w", err)
	}
	return tasks, nil
}

// ---------------------------------------------------------------------------
// startCleaningTask / completeCleaningTask
// ---------------------------------------------------------------------------

// StartTask transitions pending -> in_progress. A task owned by another
// operator is indistinguishable from a missing task (ErrTaskNotFound).
func (r *CleaningRepository) StartTask(
	ctx context.Context,
	taskID, operatorID int64,
) (*model.CleaningTask, error) {
	const startTaskUpdateQuery = `
		UPDATE cleaning_tasks
		   SET status = 'in_progress'
		 WHERE id = $1
		RETURNING id, shift_id, dorm_unit_id, cleaning_request_id,
		          task_order, status, completed_at, notes, created_at, updated_at;
	`

	return r.transitionTask(ctx, taskID, operatorID,
		model.CleaningTaskStatusPending, startTaskUpdateQuery,
	)
}

// CompleteTask transitions in_progress -> done and stamps completed_at.
func (r *CleaningRepository) CompleteTask(
	ctx context.Context,
	taskID, operatorID int64,
) (*model.CleaningTask, error) {
	const completeTaskUpdateQuery = `
		UPDATE cleaning_tasks
		   SET status = 'done',
		       completed_at = now()
		 WHERE id = $1
		RETURNING id, shift_id, dorm_unit_id, cleaning_request_id,
		          task_order, status, completed_at, notes, created_at, updated_at;
	`
	return r.transitionTask(ctx, taskID, operatorID,
		model.CleaningTaskStatusInProgress, completeTaskUpdateQuery,
	)
}

func (r *CleaningRepository) transitionTask(
	ctx context.Context,
	taskID, operatorID int64,
	expectedStatus, updateQuery string,
) (*model.CleaningTask, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin task transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	
	const getTaskForUpdateQuery = `
		SELECT t.id, t.shift_id, t.dorm_unit_id, t.cleaning_request_id,
		       t.task_order, t.status, t.completed_at, t.notes,
		       t.created_at, t.updated_at
		FROM cleaning_tasks t
		JOIN cleaning_shifts s ON s.id = t.shift_id
		WHERE t.id = $1
		  AND s.operator_id = $2
		FOR UPDATE OF t;
	`
	
	task, err := scanTask(tx.QueryRow(ctx, getTaskForUpdateQuery, taskID, operatorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrTaskNotFound
		}
		return nil, fmt.Errorf("load task for transition: %w", err)
	}
	if task.Status != expectedStatus {
		return nil, model.ErrInvalidTaskTransition
	}

	updated, err := scanTask(tx.QueryRow(ctx, updateQuery, taskID))
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit task transition: %w", err)
	}
	return updated, nil
}

// ---------------------------------------------------------------------------
// assignHousekeeper
// ---------------------------------------------------------------------------

// AssignHousekeeper binds an operator to a residence.
//
// NOTE: user_id is an Identity user id. The manager's JWT cannot prove the
// target's worker_role, so the roster is trusted input for now; a future
// internal Identity endpoint (or a local cache) must validate it. The
// database still enforces one residence per operator (UNIQUE(user_id)).
func (r *CleaningRepository) AssignHousekeeper(
	ctx context.Context,
	userID, residenceID int64,
) (*model.HousekeeperAssignment, error) {
	var a model.HousekeeperAssignment
	const assignHousekeeperQuery = `
		INSERT INTO housekeeper_assignments (user_id, residence_id)
		VALUES ($1, $2)
		RETURNING id, user_id, residence_id, created_at;
	`
	err := r.pool.QueryRow(ctx, assignHousekeeperQuery, userID, residenceID).Scan(
		&a.ID, &a.UserID, &a.ResidenceID, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("assign housekeeper: %w", mapAssignmentError(err))
	}
	return &a, nil
}

// ---------------------------------------------------------------------------
// Scanners
// ---------------------------------------------------------------------------

func scanCleaningRequest(row pgx.Row) (*model.CleaningRequest, error) {
	var req model.CleaningRequest
	err := row.Scan(
		&req.ID, &req.DormUnitID, &req.StudentID, &req.StayID,
		&req.ScheduledDate, &req.Slot, &req.Status, &req.Year, &req.ISOWeek,
		&req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func scanTask(row pgx.Row) (*model.CleaningTask, error) {
	var t model.CleaningTask
	err := row.Scan(
		&t.ID, &t.ShiftID, &t.DormUnitID, &t.CleaningRequestID,
		&t.TaskOrder, &t.Status, &t.CompletedAt, &t.Notes,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// PostgreSQL error mapping
// ---------------------------------------------------------------------------

func mapCleaningRequestError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "uq_unit_weekly_cleaning_quota":
			return model.ErrWeeklyQuotaExceeded
		case pgErr.Code == "23514" || strings.HasPrefix(pgErr.Code, "22"):
			return model.ErrInvalidInput
		}
	}
	return err
}

func mapAssignmentError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "uq_housekeeper_assignments":
			return model.ErrAssignmentExists
		case pgErr.Code == "23503":
			// housekeeper_assignments.residence_id -> residences(id)
			return model.ErrResidenceNotFound
		case pgErr.Code == "23514" || strings.HasPrefix(pgErr.Code, "22"):
			return model.ErrInvalidInput
		}
	}
	return err
}
