package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

type fakeCleaningRepo struct {
	called bool

	gotStudentID int64
	gotDate      time.Time
	gotSlot      model.CleaningSlot
	gotYear      int
	gotWeek      int

	result *model.CleaningRequest
	err    error
}

func (f *fakeCleaningRepo) CreateFromActiveStay(
	_ context.Context,
	studentID int64,
	scheduledDate time.Time,
	slot model.CleaningSlot,
	isoYear, isoWeek int,
) (*model.CleaningRequest, error) {
	f.called = true
	f.gotStudentID = studentID
	f.gotDate = scheduledDate
	f.gotSlot = slot
	f.gotYear = isoYear
	f.gotWeek = isoWeek
	return f.result, f.err
}

func fixedService(repo *fakeCleaningRepo, now time.Time) *CleaningService {
	loc := time.FixedZone("Casa", 3600)
	svc := NewCleaningService(repo, loc)
	svc.now = func() time.Time { return now }
	return svc
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCreateCleaningRequestDefaultsToTomorrow(t *testing.T) {
	repo := &fakeCleaningRepo{result: &model.CleaningRequest{ID: 1}}
	svc := fixedService(repo, time.Date(2026, 12, 31, 10, 0, 0, 0, time.FixedZone("Casa", 3600)))

	got, err := svc.CreateCleaningRequest(context.Background(), 42, nil, model.CleaningSlotMorning)
	if err != nil {
		t.Fatalf("CreateCleaningRequest() error = %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Fatalf("CreateCleaningRequest() = %+v, want the repository result", got)
	}
	if !repo.called {
		t.Fatal("repository was not called")
	}
	if repo.gotStudentID != 42 {
		t.Errorf("student id = %d, want 42", repo.gotStudentID)
	}
	if want := date(2027, time.January, 1); !repo.gotDate.Equal(want) {
		t.Errorf("scheduled date = %s, want %s", repo.gotDate, want)
	}
	if repo.gotSlot != model.CleaningSlotMorning {
		t.Errorf("slot = %q, want morning", repo.gotSlot)
	}
	// 1 Jan 2027 belongs to ISO week 53 of 2026.
	if repo.gotYear != 2026 || repo.gotWeek != 53 {
		t.Errorf("ISO year/week = %d/%d, want 2026/53", repo.gotYear, repo.gotWeek)
	}
}

func TestCreateCleaningRequestAcceptsTomorrow(t *testing.T) {
	repo := &fakeCleaningRepo{result: &model.CleaningRequest{ID: 1}}
	svc := fixedService(repo, time.Date(2026, 12, 31, 10, 0, 0, 0, time.FixedZone("Casa", 3600)))
	tomorrow := date(2027, time.January, 1)

	if _, err := svc.CreateCleaningRequest(context.Background(), 42, &tomorrow, model.CleaningSlotAfternoon); err != nil {
		t.Fatalf("CreateCleaningRequest() error = %v", err)
	}
	if !repo.gotDate.Equal(tomorrow) {
		t.Errorf("scheduled date = %s, want %s", repo.gotDate, tomorrow)
	}
	if repo.gotSlot != model.CleaningSlotAfternoon {
		t.Errorf("slot = %q, want afternoon", repo.gotSlot)
	}
}

func TestCreateCleaningRequestRejectsNonTomorrow(t *testing.T) {
	now := time.Date(2026, 12, 31, 10, 0, 0, 0, time.FixedZone("Casa", 3600))

	tests := []struct {
		name string
		date time.Time
	}{
		{"today", date(2026, time.December, 31)},
		{"yesterday", date(2026, time.December, 30)},
		{"next week", date(2027, time.January, 7)},
		{"far future", date(2027, time.February, 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeCleaningRepo{}
			svc := fixedService(repo, now)

			_, err := svc.CreateCleaningRequest(context.Background(), 42, &tt.date, model.CleaningSlotMorning)
			if !errors.Is(err, model.ErrNotTomorrow) {
				t.Fatalf("error = %v, want ErrNotTomorrow", err)
			}
			if repo.called {
				t.Fatal("repository must not be called for a rejected date")
			}
		})
	}
}

func TestCreateCleaningRequestRejectsInvalidSlot(t *testing.T) {
	repo := &fakeCleaningRepo{}
	svc := fixedService(repo, time.Date(2026, 12, 31, 10, 0, 0, 0, time.FixedZone("Casa", 3600)))

	_, err := svc.CreateCleaningRequest(context.Background(), 42, nil, model.CleaningSlot("evening"))
	if !errors.Is(err, model.ErrInvalidSlot) {
		t.Fatalf("error = %v, want ErrInvalidSlot", err)
	}
	if repo.called {
		t.Fatal("repository must not be called for an invalid slot")
	}
}

func TestCreateCleaningRequestPropagatesRepositoryError(t *testing.T) {
	repo := &fakeCleaningRepo{err: model.ErrNoActiveStay}
	svc := fixedService(repo, time.Date(2026, 12, 31, 10, 0, 0, 0, time.FixedZone("Casa", 3600)))

	_, err := svc.CreateCleaningRequest(context.Background(), 42, nil, model.CleaningSlotMorning)
	if !errors.Is(err, model.ErrNoActiveStay) {
		t.Fatalf("error = %v, want ErrNoActiveStay", err)
	}
}

func TestCreateCleaningRequestUsesLocalDateAtTimezoneBoundary(t *testing.T) {
	// 23:30 UTC on 31 Dec is already 00:30 on 1 Jan in Casa. "Tomorrow"
	// must therefore be 2 Jan, not 1 Jan.
	repo := &fakeCleaningRepo{result: &model.CleaningRequest{ID: 1}}
	svc := fixedService(repo, time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC))

	if _, err := svc.CreateCleaningRequest(context.Background(), 42, nil, model.CleaningSlotMorning); err != nil {
		t.Fatalf("CreateCleaningRequest() error = %v", err)
	}
	if want := date(2027, time.January, 2); !repo.gotDate.Equal(want) {
		t.Errorf("scheduled date = %s, want %s", repo.gotDate, want)
	}
}
