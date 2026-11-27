package repository

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

// integrationPool returns a pool for TEST_DATABASE_URL. The test is skipped
// when the variable is absent, so `go test ./...` stays offline-friendly.
//
// WARNING: the test truncates users and sessions. Only point
// TEST_DATABASE_URL at a disposable database.
func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("pinging test database: %v", err)
	}
	return pool
}

func randomHash(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generating token hash: %v", err)
	}
	return b
}

func TestSessionLifecycleIntegration(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE sessions, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating test tables: %v", err)
	}

	var userID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO users (
			first_name, last_name, email, phone_number,
			affiliation, role, password_hash, status, campus_id
		) VALUES (
			'Test', 'User', $1, '+212600009999',
			'um6p', 'student', 'hash', 'active',
			(SELECT id FROM campuses WHERE code = 'rabat')
		)
		RETURNING id;
	`, "it-"+uuid.NewString()+"@example.com").Scan(&userID)
	if err != nil {
		t.Fatalf("inserting test user: %v", err)
	}

	repo := NewSessionRepository(pool)
	newSession := func() *model.Session {
		return &model.Session{
			UserID:    userID,
			TokenHash: randomHash(t),
			IPAddress: "127.0.0.1",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
			FamilyID:  uuid.New(),
		}
	}

	first := newSession()
	second := newSession()
	third := newSession()

	if err := repo.CreateWithLimit(ctx, first, 2); err != nil {
		t.Fatalf("creating first session: %v", err)
	}
	if first.LastUsedAt == nil {
		t.Error("first session last_used_at is nil")
	}
	if err := repo.CreateWithLimit(ctx, second, 2); err != nil {
		t.Fatalf("creating second session: %v", err)
	}
	if err := repo.CreateWithLimit(ctx, third, 2); err != nil {
		t.Fatalf("creating third session: %v", err)
	}

	sessions, err := repo.ListActiveByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("listing active sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("active sessions = %d, want 2", len(sessions))
	}
	if sessions[0].LastUsedAt == nil {
		t.Error("active session last_used_at is nil")
	}

	revoked, err := repo.RevokeByIDForUser(ctx, userID, sessions[0].ID)
	if err != nil {
		t.Fatalf("revoking own session: %v", err)
	}
	if !revoked {
		t.Error("expected own session to be revoked")
	}

	revoked, err = repo.RevokeByIDForUser(ctx, userID+999, sessions[1].ID)
	if err != nil {
		t.Fatalf("revoking another user's session: %v", err)
	}
	if revoked {
		t.Error("session of another user must not be revoked")
	}

	// The first session was pushed out by the limit; make it old enough for GC.
	if _, err := pool.Exec(ctx, `
		UPDATE sessions
		   SET created_at = now() - interval '40 days',
		       expires_at = now() - interval '31 days'
		 WHERE id = $1;
	`, first.ID); err != nil {
		t.Fatalf("aging first session: %v", err)
	}
	deleted, err := repo.DeleteStale(ctx, time.Now().UTC().Add(-30*24*time.Hour), 100)
	if err != nil {
		t.Fatalf("deleting stale sessions: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted stale sessions = %d, want 1", deleted)
	}
}
