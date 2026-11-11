// Package repository provides Postgres data access for users and sessions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, s *model.Session) error {
	const query = `
		INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at, family_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at;
		`
	err := r.pool.QueryRow(ctx, query,
		s.UserID,
		s.TokenHash,
		s.IPAddress,
		s.UserAgent,
		s.ExpiresAt,
		s.FamilyID,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("session repo create: %w", err)
	}
	return nil
}

func (r *SessionRepository) FindByTokenHash(ctx context.Context, tokenHash []byte) (*model.Session, error) {
	const query = `
		SELECT id, user_id, token_hash, ip_address::text, user_agent, expires_at,
		       revoked_at, last_used_at, family_id, rotated_at, replaced_by,
		       created_at, updated_at
		FROM sessions
		WHERE token_hash = $1;
		`
	var s model.Session

	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&s.ID,
		&s.UserID,
		&s.TokenHash,
		&s.IPAddress,
		&s.UserAgent,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.LastUsedAt,
		&s.FamilyID,
		&s.RotatedAt,
		&s.ReplacedBy,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrSessionNotFound
		}
		return nil, fmt.Errorf("session repo find by token hash: %w", err)

	}
	return &s, nil
}

func (r *SessionRepository) Rotate(ctx context.Context, oldSessionID int64, newSession *model.Session) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("session repo rotate begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// INSERT the new session first so we have its ID for the replaced_by FK.
	const insertQuery = `
		INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at, family_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, insertQuery,
		newSession.UserID,
		newSession.TokenHash,
		newSession.IPAddress,
		newSession.UserAgent,
		newSession.ExpiresAt,
		newSession.FamilyID,
	).Scan(&newSession.ID, &newSession.CreatedAt, &newSession.UpdatedAt)
	if err != nil {
		return fmt.Errorf("session repo rotate insert: %w", err)
	}

	// Now mark the old session rotated, pointing replaced_by at the new ID.
	const updateQuery = `
		UPDATE sessions
		   SET revoked_at   = now(),
		       rotated_at   = now(),
		       last_used_at = now(),
		       replaced_by  = $1
		 WHERE id = $2
		   AND revoked_at IS NULL;
	`
	cmdTag, err := tx.Exec(ctx, updateQuery, newSession.ID, oldSessionID)
	if err != nil {
		return fmt.Errorf("session repo rotate update old: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return model.ErrSessionNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("session repo rotate commit: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID int64) error{
	const query = `
			UPDATE sessions
				SET revoked_at = now(),
					updated_at = now()
			WHERE id = $1
				AND revoked_at IS NULL;
			`
	_, err := r.pool.Exec(ctx, query, sessionID)
	if err != nil{
		return fmt.Errorf("session reop revoke by id: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeAllByUserID(ctx context.Context, userID int64) error{
	const query = `
        UPDATE sessions
           SET revoked_at = now(),
               updated_at = now()
         WHERE user_id = $1
           AND revoked_at IS NULL;
    `
    _, err := r.pool.Exec(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("session repo revoke all by user id: %w", err)
    }
    return nil
}

func (r *SessionRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error{
	const query = `
        UPDATE sessions
           SET revoked_at = now(),
               updated_at = now()
         WHERE family_id = $1
           AND revoked_at IS NULL;
    `
    _, err := r.pool.Exec(ctx, query, familyID)
    if err != nil {
        return fmt.Errorf("session repo revoke family: %w", err)
    }
    return nil
}
