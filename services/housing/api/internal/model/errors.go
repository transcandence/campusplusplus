package model

import "errors"

// Domain errors returned by the repository and translated to HTTP status
// codes by the handler. They are deliberately small and stable: the handler
// maps them to Problem Details codes, not to raw database errors.
var (
	// ErrNoActiveStay means the student has no stay with status = 'active'.
	// POST /cleaning-requests maps it to 422 (the contract declares no 404
	// for that endpoint).
	ErrNoActiveStay = errors.New("no active stay")

	// ErrNotTomorrow means scheduledDate is anything other than the next
	// civil day in the service timezone. Mapped to 422.
	ErrNotTomorrow = errors.New("scheduled date must be tomorrow")

	// ErrInvalidSlot means the requested half-day is not morning/afternoon.
	// Mapped to 422 with a field-level error.
	ErrInvalidSlot = errors.New("invalid cleaning slot")

	// ErrWeeklyQuotaExceeded means the unit already has a live cleaning
	// request for the same ISO week. Mapped to 409.
	ErrWeeklyQuotaExceeded = errors.New("weekly cleaning quota exceeded")

	// ErrTaskNotFound also covers "the task belongs to another operator":
	// the caller must not be able to distinguish the two (404).
	ErrTaskNotFound = errors.New("cleaning task not found")

	// ErrInvalidTaskTransition means the task is not in the expected state
	// for the requested transition. Mapped to 409.
	ErrInvalidTaskTransition = errors.New("invalid cleaning task transition")

	// ErrAssignmentExists means the operator is already assigned to a
	// residence (UNIQUE(user_id)). Mapped to 409.
	ErrAssignmentExists = errors.New("housekeeper assignment already exists")

	// ErrResidenceNotFound means the residence id does not exist. Mapped to
	// 422 for assignHousekeeper (the contract declares no 404 there).
	ErrResidenceNotFound = errors.New("residence not found")

	// ErrInvalidInput covers database CHECK/data-exception violations that
	// indicate a bad request value. Mapped to 422.
	ErrInvalidInput = errors.New("invalid input")
)
