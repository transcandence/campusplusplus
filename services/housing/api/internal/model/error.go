package model

import (
	"errors"
	"net/http"
)

var (
	ErrStayNotFound        = errors.New("stay not found")
	ErrUnitNotFound        = errors.New("dorm unit not found")
	ErrUnitNotLiving       = errors.New("dorm unit is not a living unit")
	ErrUnitFull            = errors.New("dorm unit has no free capacity")
	ErrIllegalTransition   = errors.New("illegal stay state transition")
	ErrDuplicateActiveStay = errors.New("student already has an active stay")
)

// AppError is the only error the HTTP layer knows how to render.
type AppError struct {
	Status int
	Code   string
	Detail string
	Field  string
}

func (e *AppError) Error() string { return e.Code + ": " + e.Detail }

func NotFound(code, detail string) *AppError {
	return &AppError{http.StatusNotFound, code, detail, ""}
}

func Forbidden(detail string) *AppError {
	return &AppError{http.StatusForbidden, "forbidden", detail, ""}
}

func Conflict(code, detail string) *AppError {
	return &AppError{http.StatusConflict, code, detail, ""}
}

func Invalid(field, code, detail string) *AppError {
	return &AppError{http.StatusUnprocessableEntity, code, detail, field}
}

func Unavailable(code, detail string) *AppError {
	return &AppError{http.StatusServiceUnavailable, code, detail, ""}
}
