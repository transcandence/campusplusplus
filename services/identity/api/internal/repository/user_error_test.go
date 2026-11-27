package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

func TestMapUserWriteError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "duplicate", err: &pgconn.PgError{Code: "23505"}, want: model.ErrUserAlreadyExists},
		{name: "check violation", err: &pgconn.PgError{Code: "23514"}, want: model.ErrInvalidInput},
		{name: "invalid text representation", err: &pgconn.PgError{Code: "22P02"}, want: model.ErrInvalidInput},
		{name: "string too long", err: &pgconn.PgError{Code: "22001"}, want: model.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapUserWriteError(tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapUserWriteError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestMapUserWriteErrorLeavesUnknownErrorsAlone(t *testing.T) {
	original := errors.New("connection refused")
	if got := mapUserWriteError(original); !errors.Is(got, original) {
		t.Fatalf("mapUserWriteError() = %v, want original error", got)
	}
}
