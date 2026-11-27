// Package repository provides Postgres data access for users and sessions.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

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

const insertSessionQuery = `
	INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at, family_id, last_used_at)
	VALUES ($1, $2, $3, $4, $5, $6, now())
	RETURNING id, created_at, updated_at, last_used_at;
`

func scanInsertedSession(row pgx.Row, s *model.Session) error {
	return row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt, &s.LastUsedAt)
}

func (r *SessionRepository) Create(ctx context.Context, s *model.Session) error {
	err := scanInsertedSession(r.pool.QueryRow(ctx, insertSessionQuery,
		s.UserID,
		s.TokenHash,
		s.IPAddress,
		s.UserAgent,
		s.ExpiresAt,
		s.FamilyID,
	), s)
	if err != nil {
		return fmt.Errorf("session repo create: %w", err)
	}
	return nil
}

// CreateWithLimit creates a session and revokes the oldest active sessions
// above maxActive in the same transaction. An advisory lock serializes
// concurrent logins for the same user so the limit cannot be exceeded by a
// race. maxActive <= 0 disables the limit.
func (r *SessionRepository) CreateWithLimit(ctx context.Context, s *model.Session, maxActive int) error {
	if maxActive <= 0 {
		return r.Create(ctx, s)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("session repo create with limit begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, s.UserID); err != nil {
		return fmt.Errorf("session repo create with limit lock: %w", err)
	}

	if err := scanInsertedSession(tx.QueryRow(ctx, insertSessionQuery,
		s.UserID,
		s.TokenHash,
		s.IPAddress,
		s.UserAgent,
		s.ExpiresAt,
		s.FamilyID,
	), s); err != nil {
		return fmt.Errorf("session repo create with limit insert: %w", err)
	}

	const revokeExtraQuery = `
		WITH ranked AS (
			SELECT id,
			       row_number() OVER (
			           PARTITION BY user_id
			           ORDER BY created_at DESC, id DESC
			       ) AS rn
			  FROM sessions
			 WHERE user_id = $1
			   AND revoked_at IS NULL
		)
		UPDATE sessions
		   SET revoked_at = now(),
		       updated_at = now()
		 WHERE id IN (SELECT id FROM ranked WHERE rn > $2);
	`
	if _, err := tx.Exec(ctx, revokeExtraQuery, s.UserID, maxActive); err != nil {
		return fmt.Errorf("session repo create with limit revoke extra: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("session repo create with limit commit: %w", err)
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

	const updateQuery = `
		UPDATE sessions
		   SET revoked_at   = now(),
		       rotated_at   = now(),
		       last_used_at = now()
		 WHERE id = $1
		   AND revoked_at IS NULL;
	`
	cmdTag, err := tx.Exec(ctx, updateQuery, oldSessionID)
	if err != nil {
		return fmt.Errorf("session repo rotate update old: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return model.ErrSessionNotFound
	}

	err = scanInsertedSession(tx.QueryRow(ctx, insertSessionQuery,
		newSession.UserID,
		newSession.TokenHash,
		newSession.IPAddress,
		newSession.UserAgent,
		newSession.ExpiresAt,
		newSession.FamilyID,
	), newSession)
	if err != nil {
		return fmt.Errorf("session repo rotate insert: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET replaced_by = $1 WHERE id = $2`,
		newSession.ID, oldSessionID); err != nil {
		return fmt.Errorf("session repo rotate link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("session repo rotate commit: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID int64) error {
	const query = `
			UPDATE sessions
				SET revoked_at = now(),
					updated_at = now()
			WHERE id = $1
				AND revoked_at IS NULL;
			`
	_, err := r.pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("session repo revoke by id: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeAllByUserID(ctx context.Context, userID int64) error {
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

func (r *SessionRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
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

// RevokeByIDForUser revokes one active session only if it belongs to userID.
// It returns false when the session does not exist, is already revoked, or
// belongs to another user. The caller should answer 404 in that case.
func (r *SessionRepository) RevokeByIDForUser(ctx context.Context, userID, sessionID int64) (bool, error) {
	const query = `
		UPDATE sessions
		   SET revoked_at = now(),
		       updated_at = now()
		 WHERE id = $1
		   AND user_id = $2
		   AND revoked_at IS NULL;
	`
	cmdTag, err := r.pool.Exec(ctx, query, sessionID, userID)
	if err != nil {
		return false, fmt.Errorf("session repo revoke by id for user: %w", err)
	}
	return cmdTag.RowsAffected() > 0, nil
}

// ListActiveByUserID returns the safe, client-facing session summary for all
// non-revoked, non-expired sessions of a user.
func (r *SessionRepository) ListActiveByUserID(ctx context.Context, userID int64) ([]model.SessionSummary, error) {
	const query = `
		SELECT id, created_at, expires_at, last_used_at,
		       ip_address::text, user_agent
		  FROM sessions
		 WHERE user_id = $1
		   AND revoked_at IS NULL
		   AND expires_at > now()
		 ORDER BY created_at DESC, id DESC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("session repo list active by user id: %w", err)
	}
	defer rows.Close()

	sessions := make([]model.SessionSummary, 0)
	for rows.Next() {
		var s model.SessionSummary
		if err := rows.Scan(
			&s.ID,
			&s.CreatedAt,
			&s.ExpiresAt,
			&s.LastUsedAt,
			&s.IPAddress,
			&s.UserAgent,
		); err != nil {
			return nil, fmt.Errorf("session repo scan active session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session repo iterate active sessions: %w", err)
	}
	return sessions, nil
}

// DeleteStale hard-deletes sessions that expired or were revoked before the
// cutoff. It is intended for a periodic cleanup job and processes at most
// limit rows per call.
func (r *SessionRepository) DeleteStale(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	const query = `
		DELETE FROM sessions
		 WHERE id IN (
		     SELECT id
		       FROM sessions
		      WHERE expires_at < $1
		         OR (revoked_at IS NOT NULL AND revoked_at < $1)
		      ORDER BY expires_at
		      LIMIT $2
		 );
	`
	cmdTag, err := r.pool.Exec(ctx, query, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("session repo delete stale: %w", err)
	}
	return cmdTag.RowsAffected(), nil
}
