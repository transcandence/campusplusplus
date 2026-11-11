// Command seed populates the identity database with predictable dev data.
// Safe to run repeatedly (idempotent).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/platform/go/auth"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/config"
)

type seedUser struct {
	FirstName   string
	LastName    string
	Email       string
	Phone       string
	Affiliation string
	Role        string
	Gender      string
	Password    string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	// 1. Resolve default campus ID (rabat)
	var campusID int64
	err = pool.QueryRow(ctx, "SELECT id FROM campuses WHERE code = 'rabat'").Scan(&campusID)
	if err != nil {
		return fmt.Errorf("resolving rabat campus id: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 2. Define seed users
	users := []seedUser{
		{
			FirstName:   "Sara",
			LastName:    "Benali",
			Email:       "sara.benali@campus.ma",
			Phone:       "+212600000001",
			Affiliation: "1337",
			Role:        "student",
			Gender:      "female",
			Password:    "Sara@benali1234",
		},
		{
			FirstName:   "Karim",
			LastName:    "Alami",
			Email:       "worker@campus.ma",
			Phone:       "+212600000002",
			Affiliation: "um6p",
			Role:        "worker",
			Gender:      "male",
			Password:    "Worker@123456",
		},
		{
			FirstName:   "Admin",
			LastName:    "System",
			Email:       "admin@campus.ma",
			Phone:       "+212600000003",
			Affiliation: "um6p",
			Role:        "admin",
			Gender:      "other",
			Password:    "Admin@123456",
		},
	}

	const upsertUserQuery = `
		INSERT INTO users (
			first_name, last_name, email, phone_number,
			affiliation, role, gender, password_hash,
			status, campus_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9)
		ON CONFLICT (email) DO UPDATE SET
			first_name    = EXCLUDED.first_name,
			last_name     = EXCLUDED.last_name,
			phone_number  = EXCLUDED.phone_number,
			affiliation   = EXCLUDED.affiliation,
			role          = EXCLUDED.role,
			gender        = EXCLUDED.gender,
			password_hash = EXCLUDED.password_hash,
			status        = 'active',
			campus_id     = EXCLUDED.campus_id,
			updated_at    = now()
		RETURNING id;
	`

	log.Println("Seeding identity users...")

	for _, u := range users {
		hash, err := auth.HashPassword(u.Password)
		if err != nil {
			return fmt.Errorf("hashing password for %s: %w", u.Email, err)
		}

		var userID int64
		err = tx.QueryRow(ctx, upsertUserQuery,
			u.FirstName,
			u.LastName,
			u.Email,
			u.Phone,
			u.Affiliation,
			u.Role,
			u.Gender,
			hash,
			campusID,
		).Scan(&userID)
		if err != nil {
			return fmt.Errorf("upserting user %s: %w", u.Email, err)
		}

		log.Printf("✔ [%s] %s (id: %d, role: %s, password: %s)", u.Role, u.Email, userID, u.Role, u.Password)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed tx: %w", err)
	}

	log.Println("Seeding completed successfully.")
	return nil
}
