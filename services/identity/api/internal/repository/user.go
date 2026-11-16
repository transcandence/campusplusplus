// Package repository provides Postgres data access for users and sessions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.TrimSpace(email)
	const query = `
		SELECT	id, first_name, last_name, email, phone_number, birthday, affiliation, role, gender, password_hash, nationality, status, campus_id, created_at, updated_at
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
		       affiliation, role, gender, password_hash, nationality, status,
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
		SELECT u.id, u.email, u.first_name, u.last_name, u.affiliation, u.role, u.status, c.code, u.created_at
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
