// Package model contains the housing domain types shared by the repository,
// service and handler layers. It intentionally does not import the generated
// handler types: the HTTP DTOs are mapped at the edge.
package model

import "time"

// CleaningSlot is the half-day booking bucket chosen by the student. It is a
// discrete enum, not a timestamp: the actual working hours live on
// cleaning_shifts.start_time/end_time.
type CleaningSlot string

const (
	CleaningSlotMorning   CleaningSlot = "morning"
	CleaningSlotAfternoon CleaningSlot = "afternoon"
)

func (s CleaningSlot) Valid() bool {
	switch s {
	case CleaningSlotMorning, CleaningSlotAfternoon:
		return true
	}
	return false
}

// CleaningRequest is a student's on-demand cleaning booking for their unit.
// ScheduledDate is a civil date stored at UTC midnight (PostgreSQL DATE).
type CleaningRequest struct {
	ID            int64
	DormUnitID    int64
	StudentID     int64
	StayID        int64
	ScheduledDate time.Time
	Slot          CleaningSlot
	Status        string
	Year          int
	ISOWeek       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CleaningShift is one operator's working day inside a campus.
type CleaningShift struct {
	ID           int64
	OperatorID   int64
	SupervisorID int64
	CampusID     int64
	ShiftDate    time.Time
	StartTime    string
	EndTime      string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CleaningTask is one unit to clean inside a shift. CleaningRequestID is nil
// for common-area tasks.
type CleaningTask struct {
	ID                int64
	ShiftID           int64
	DormUnitID        int64
	CleaningRequestID *int64
	TaskOrder         int
	Status            string
	CompletedAt       *time.Time
	Notes             *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// HousekeeperAssignment binds an operator (Identity user id) to one residence.
type HousekeeperAssignment struct {
	ID          int64
	UserID      int64
	ResidenceID int64
	CreatedAt   time.Time
}

// TaskFilter carries the optional listCleaningTasks query filters. Nil means
// "no filter". The caller must never derive its authorization scope from these
// values: scope is a separate, mandatory argument of the repository method.
type TaskFilter struct {
	ShiftID    *int64
	Status     *string
	DormUnitID *int64
}

const (
	CleaningRequestStatusPending   = "pending"
	CleaningRequestStatusScheduled = "scheduled"
	CleaningRequestStatusCompleted = "completed"
	CleaningRequestStatusCancelled = "cancelled"

	CleaningTaskStatusPending    = "pending"
	CleaningTaskStatusInProgress = "in_progress"
	CleaningTaskStatusDone       = "done"
	CleaningTaskStatusSkipped    = "skipped"
)
