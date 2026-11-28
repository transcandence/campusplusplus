// Package service contains the housing business rules. It depends on narrow
// repository interfaces so it can be tested without a database.
package service

import (
	"context"
	"time"

	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

// CleaningRepository is the persistence subset needed by CreateCleaningRequest.
// The concrete repository.CleaningRepository satisfies it.
type CleaningRepository interface {
	CreateFromActiveStay(
		ctx context.Context,
		studentID int64,
		scheduledDate time.Time,
		slot model.CleaningSlot,
		isoYear, isoWeek int,
	) (*model.CleaningRequest, error)
}

// CleaningService implements the cleaning use cases.
type CleaningService struct {
	repo CleaningRepository
	loc  *time.Location
	now  func() time.Time
}

// NewCleaningService builds the service. loc is the civil timezone used to
// resolve "today" and "tomorrow" (Africa/Casablanca in production). It is
// injected so tests are deterministic.
func NewCleaningService(repo CleaningRepository, loc *time.Location) *CleaningService {
	if loc == nil {
		loc = time.UTC
	}
	return &CleaningService{
		repo: repo,
		loc:  loc,
		now:  time.Now,
	}
}

// CreateCleaningRequest validates the slot, resolves the scheduled date to
// strictly tomorrow (defaulting when omitted), computes the ISO year/week and
// delegates the atomic insert to the repository.
func (s *CleaningService) CreateCleaningRequest(
	ctx context.Context,
	studentID int64,
	scheduledDate *time.Time,
	slot model.CleaningSlot,
) (*model.CleaningRequest, error) {
	if !slot.Valid() {
		return nil, model.ErrInvalidSlot
	}

	date, err := s.resolveScheduledDate(scheduledDate)
	if err != nil {
		return nil, err
	}

	isoYear, isoWeek := date.ISOWeek()
	return s.repo.CreateFromActiveStay(ctx, studentID, date, slot, isoYear, isoWeek)
}

// resolveScheduledDate returns the scheduled civil date at UTC midnight.
//
// If input is nil, it returns tomorrow in s.loc. If input is provided, only
// that same civil date is accepted; everything else (today, yesterday, next
// week, …) is ErrNotTomorrow.
func (s *CleaningService) resolveScheduledDate(input *time.Time) (time.Time, error) {
	now := s.now().In(s.loc)

	// Build the civil date from local Y/M/D. Using UTC for the result keeps
	// the value a pure date and avoids any DST/session-timezone shift.
	ty, tm, td := now.Date()
	tomorrow := time.Date(ty, tm, td+1, 0, 0, 0, 0, time.UTC)

	if input == nil {
		return tomorrow, nil
	}

	// input is a civil date (oapi-codegen Date is UTC midnight). Read its
	// calendar fields, not its instant, then compare civil dates.
	y, m, d := input.UTC().Date()
	requested := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if !requested.Equal(tomorrow) {
		return time.Time{}, model.ErrNotTomorrow
	}
	return requested, nil
}
