
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func migrationup(conn *pgx.Conn, level string) {
	fmt.Println("Running migration up to level", level)

	files, err := os.ReadDir("../../migrations")
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	var migrations []string

	for _, file := range files {
		filename := file.Name()

		if !file.IsDir() && strings.HasSuffix(filename, ".up.sql") {
			migrations = append(migrations, filename)
		}
	}

	var mig string

	for _, migration := range migrations {
		if strings.Contains(migration, level) {
			mig = migration
			break
		}
	}

	if mig == "" {
		fmt.Println("Migration not found:", level)
		os.Exit(1)
	}

	sqlfile, err := os.ReadFile("../../migrations/" + mig)
	if err != nil {
		fmt.Println("Error reading SQL file:", err)
		os.Exit(1)
	}

	_, err = conn.Exec(context.Background(), string(sqlfile))
	if err != nil {
		fmt.Println("Migration failed:", err)
		os.Exit(1)
	}

	fmt.Println("Identity migration up created successfully!")
}

func migrationdown(conn *pgx.Conn, level string) {
	fmt.Println("Running migration down to level", level)

	files, err := os.ReadDir("../../migrations")
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	var migrations []string

	for _, file := range files {
		filename := file.Name()

		if !file.IsDir() && strings.HasSuffix(filename, ".down.sql") {
			migrations = append(migrations, filename)
		}
	}

	var mig string

	for _, migration := range migrations {
		if strings.Contains(migration, level) {
			mig = migration
			break
		}
	}

	if mig == "" {
		fmt.Println("Migration not found:", level)
		os.Exit(1)
	}

	sqlfile, err := os.ReadFile("../../migrations/" + mig)
	if err != nil {
		fmt.Println("Error reading SQL file:", err)
		os.Exit(1)
	}

	_, err = conn.Exec(context.Background(), string(sqlfile))
	if err != nil {
		fmt.Println("Migration failed:", err)
		os.Exit(1)
	}

	fmt.Println("Identity migration down executed successfully!")
}

func main() {
	args := os.Args

	if len(args) != 3 {
		fmt.Println("Usage: program <up|down> <level>")
		os.Exit(1)
	}

	if args[1] != "up" && args[1] != "down" {
		fmt.Println("Invalid command. Use 'up' or 'down'.")
		os.Exit(1)
	}

	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Println("Error loading .env:", err)
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Println("Error: DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		fmt.Println("Invalid database URL:", err)
		os.Exit(1)
	}

	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		fmt.Println("Database connection failed:", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	fmt.Println("Connected to PostgreSQL!")

	switch args[1] {
	case "up":
		migrationup(conn, args[2])
	case "down":
		migrationdown(conn, args[2])
	}
}
