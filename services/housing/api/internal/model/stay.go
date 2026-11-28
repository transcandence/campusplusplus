package model

import "time"

type StayStatus string

const (
	StayStatusPlanned   StayStatus = "planned"
	StayStatusActive    StayStatus = "active"
	StayStatusEnded     StayStatus = "ended"
	StayStatusCancelled StayStatus = "cancelled"
)

func IsValidStayStatus(s StayStatus) bool {
	switch s {
	case StayStatusPlanned, StayStatusActive, StayStatusEnded, StayStatusCancelled:
		return true
	}
	return false
}

type Stay struct {
	ID                 int64
	StudentID          int64
	DormUnitID         int64
	StartDate          time.Time
	EndDate            time.Time
	Status             StayStatus
	AssignedBy         int64
	CheckedInBy        *int64
	CheckedInAt        *time.Time
	CheckedOutBy       *int64
	CheckedOutAt       *time.Time
	CancelledBy        *int64
	CancelledAt        *time.Time
	CancellationReason *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateStayParams struct {
	StudentID  int64
	DormUnitID int64
	StartDate  time.Time
	EndDate    time.Time
	AssignedBy int64
}

// StayFilter holds the optional listStays filters. Nil means "not filtered".
type StayFilter struct {
	Status     *StayStatus
	StudentID  *int64
	DormUnitID *int64
	CampusID   *int64
}

// UnitWithPolicy is a dorm unit joined to its residence, used to enforce the
// living-unit, capacity and gender-policy rules atomically.
type UnitWithPolicy struct {
	ID           int64
	FloorID      int64
	UnitNumber   string
	Code         string
	UnitType     string
	Capacity     int
	GenderPolicy string
	CampusID     int64
}

func (u UnitWithPolicy) IsLivingUnit() bool {
	return u.UnitType == "room" || u.UnitType == "apart"
}
