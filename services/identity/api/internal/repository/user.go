// Package repository provides Postgres data access for users and sessions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// mapUserWriteError converts PostgreSQL constraint/data errors into the
// sentinels the HTTP layer knows how to map (409 for duplicates, 422 for
// invalid values). Everything else stays an internal error.
func mapUserWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505":
			return model.ErrUserAlreadyExists
		case pgErr.Code == "23514" || strings.HasPrefix(pgErr.Code, "22"):
			return model.ErrInvalidInput
		}
	}
	return err
}

func (r *UserRepository) CreateUserWithOutbox(ctx context.Context, user model.User, eventPayload []byte) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	queryUser := `
		INSERT INTO users (first_name, last_name, email, phone_number,
		password_hash, affiliation, role, worker_role, gender,
		nationality, status, campus_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING id
	`
	var userID int64
	err = tx.QueryRow(
		ctx,
		queryUser,
		user.FirstName,
		user.LastName,
		user.Email,
		user.PhoneNumber,
		user.PasswordHash,
		user.Affiliation,
		user.Role,
		user.WorkerRole,
		user.Gender,
		user.Nationality,
		user.Status,
		user.CampusID,
	).Scan(&userID)
	if err != nil {
		return 0, mapUserWriteError(err)
	}
	queryOutbox := `
		INSERT INTO outbox_events (aggregate_id, event_type, payload, status)
		VALUES ($1, 'user.created', $2, 'pending')
	`
	_, err = tx.Exec(ctx, queryOutbox, userID, string(eventPayload))
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *UserRepository) CheckEmailOrPhoneExists(ctx context.Context, email, phone_number *string) (bool, error) {
	query := `
		SELECT EXISTS (
		SELECT 1 FROM users
		WHERE ($1::citext IS NOT NULL AND email = $1::citext)
		OR	($2::text IS NOT NULL AND phone_number = $2)
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, email, phone_number).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.TrimSpace(email)
	const query = `
		SELECT	id, first_name, last_name, email, phone_number, birthday, affiliation, role, worker_role, gender, password_hash, nationality, status, campus_id, created_at, updated_at
		FROM users
		WHERE email = $1;
		`
	var u model.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.PhoneNumber,
		&u.Birthday,
		&u.Affiliation,
		&u.Role,
		&u.WorkerRole,
		&u.Gender,
		&u.PasswordHash,
		&u.Nationality,
		&u.Status,
		&u.CampusID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}

		return nil, fmt.Errorf("user repo find by email: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, first_name, last_name, email, phone_number, birthday,
		       affiliation, role, worker_role, gender, password_hash, nationality, status,
		       campus_id, created_at, updated_at
		FROM users
		WHERE id = $1;
	`
	var u model.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.PhoneNumber,
		&u.Birthday,
		&u.Affiliation,
		&u.Role,
		&u.WorkerRole,
		&u.Gender,
		&u.PasswordHash,
		&u.Nationality,
		&u.Status,
		&u.CampusID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("user repo find by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindMeByID(ctx context.Context, userID int64) (*model.UserMe, error) {
	const query = `
		SELECT u.id, COALESCE(u.email::text, ''), u.first_name, u.last_name, u.affiliation, u.role, u.worker_role, u.status, c.code, u.created_at
		FROM users u
		JOIN  campuses c ON c.id = u.campus_id
		WHERE u.id = $1;
		`
	var me model.UserMe
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&me.ID,
		&me.Email,
		&me.FirstName,
		&me.LastName,
		&me.Affiliation,
		&me.Role,
		&me.WorkerRole,
		&me.Status,
		&me.Campus,
		&me.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("user repo find me by id %w", err)
	}
	return &me, nil
}

// FindCampusIDByCode returns the numeric PK for a given campus code.
func (r *UserRepository) FindCampusIDByCode(ctx context.Context, code model.CampusCode) (int64, error) {
	const query = `SELECT id FROM campuses WHERE code = $1;`
	var id int64
	err := r.pool.QueryRow(ctx, query, code).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("campus code %q not found", code)
		}
		return 0, fmt.Errorf("user repo find campus id by code: %w", err)
	}
	return id, nil
}

// UpdateMe applies the provided (non-nil) fields only. COALESCE keeps the
// current value when the caller omitted a field, so a partial PUT cannot
// accidentally erase the user's gender or contact information.
func (r *UserRepository) UpdateMe(ctx context.Context, userID int64, input model.UpdateMeInput) (*model.UserMe, error) {
	const query = `
		UPDATE users
		   SET first_name   = COALESCE($2, first_name),
		       last_name    = COALESCE($3, last_name),
		       gender       = COALESCE($4::gender, gender),
		       email        = COALESCE($5::citext, email),
		       phone_number = COALESCE($6, phone_number),
		       updated_at   = now()
		 WHERE id = $1;
	`
	cmdTag, err := r.pool.Exec(ctx, query,
		userID,
		input.FirstName,
		input.LastName,
		input.Gender,
		input.Email,
		input.Phone,
	)
	if err != nil {
		return nil, fmt.Errorf("user repo update me: %w", mapUserWriteError(err))
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, model.ErrUserNotFound
	}
	return r.FindMeByID(ctx, userID)
}
