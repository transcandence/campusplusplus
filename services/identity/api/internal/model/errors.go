package model

import "net/http"

type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type ValidationProblem struct {
	Problem
	Errors []FieldError `json:"errors"`
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

const (
	ErrCodeInvalidCredentials = "invalid_credentials"
	ErrCodeInvalidRefresh     = "invalid_refresh"
	ErrCodeAccountSuspended   = "account_suspended"
	ErrCodeAccountPending     = "account_pending_verification"
	ErrCodeValidationFailed   = "validation_failed"
	ErrCodeRateLimited        = "rate_limited"
)

func NewProblem(status int, code string) Problem {
	return Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
	}
}
