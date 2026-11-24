// Package main implements the outbox processor for the identity service.
//
// It polls the outbox_events table for pending events and publishes them to
// stdout (structured JSON log) using a FOR UPDATE SKIP LOCKED pattern that
// is safe for concurrent deployments. Replace the publish step with your
// actual broker client (NATS, RabbitMQ, Kafka, …) when one is available.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outbox worker error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parsing database url: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("outbox worker started", "poll_interval", "2s", "batch_size", 10)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbox worker shutting down gracefully")
			return nil
		case <-ticker.C:
			if err := processBatch(ctx, pool, logger); err != nil {
				logger.Error("outbox batch error", "error", err)
			}
		}
	}
}

type outboxEvent struct {
	ID          int64           `json:"id"`
	AggregateID int64           `json:"aggregate_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
}

// processBatch fetches up to 10 pending events, publishes them, and marks
// them as sent — all inside a single transaction using SKIP LOCKED so that
// multiple worker replicas never double-process the same row.
func processBatch(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectQ = `
		SELECT id, aggregate_id, event_type, payload
		FROM   outbox_events
		WHERE  status = 'pending'
		ORDER  BY id
		LIMIT  10
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := tx.Query(ctx, selectQ)
	if err != nil {
		return fmt.Errorf("query pending events: %w", err)
	}
	defer rows.Close()

	var events []outboxEvent
	for rows.Next() {
		var e outboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.EventType, &e.Payload); err != nil {
			return fmt.Errorf("scanning event row: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating event rows: %w", err)
	}
	rows.Close()

	if len(events) == 0 {
		_ = tx.Rollback(ctx)
		return nil
	}

	for _, e := range events {
		// --- Publish step -------------------------------------------
		// Replace this block with your broker client call, e.g.:
		//   broker.Publish(ctx, e.EventType, e.Payload)
		// For now we emit a structured log line so the event is not lost.
		logger.Info("publishing event",
			"event_id", e.ID,
			"event_type", e.EventType,
			"aggregate_id", e.AggregateID,
		)
		// ------------------------------------------------------------
	}

	// Mark all processed events as sent in one batch UPDATE.
	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	const updateQ = `
		UPDATE outbox_events
		   SET status       = 'sent',
		       processed_at = now()
		 WHERE id = ANY($1);
	`
	if _, err := tx.Exec(ctx, updateQ, ids); err != nil {
		return fmt.Errorf("marking events as sent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	logger.Info("outbox batch processed", "count", len(events))
	return nil
}
