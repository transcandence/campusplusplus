package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

func TestMapCleaningRequestError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "weekly quota unique violation",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "uq_unit_weekly_cleaning_quota"},
			want: model.ErrWeeklyQuotaExceeded,
		},
		{
			name: "check violation",
			err:  &pgconn.PgError{Code: "23514"},
			want: model.ErrInvalidInput,
		},
		{
			name: "invalid enum value",
			err:  &pgconn.PgError{Code: "22P02"},
			want: model.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapCleaningRequestError(tt.err); !errors.Is(got, tt.want) {
				t.Fatalf("mapCleaningRequestError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapCleaningRequestErrorLeavesUnknownErrorsAlone(t *testing.T) {
	original := errors.New("connection reset")
	if got := mapCleaningRequestError(original); got != original {
		t.Fatalf("mapCleaningRequestError() = %v, want the original error", got)
	}
}

func TestMapAssignmentError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "duplicate operator assignment",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "uq_housekeeper_assignments"},
			want: model.ErrAssignmentExists,
		},
		{
			name: "unknown residence",
			err:  &pgconn.PgError{Code: "23503"},
			want: model.ErrResidenceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapAssignmentError(tt.err); !errors.Is(got, tt.want) {
				t.Fatalf("mapAssignmentError() = %v, want %v", got, tt.want)
			}
		})
	}
}
