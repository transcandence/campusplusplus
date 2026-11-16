package model

import (
	"errors"
	"net/http"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidRefresh     = errors.New("invalid or expired refresh token")
	ErrAccountSuspended   = errors.New("account is suspended")
	ErrAccountPending     = errors.New("account email pending verification")
)

const (
	MsgInvalidBody        = "Invalid request body"
	MsgEmailPasswordReq   = "Email and password are required"
	MsgInvalidCredentials = "Invalid email or password"
	MsgAccountSuspended   = "This account is suspended. Contact the administration."
	MsgAccountPending     = "Verify your email address before logging in."
	MsgRefreshTokenReq    = "Refresh token is required"
	MsgInvalidRefresh     = "The refresh token is no longer usable. Log in again."
	MsgUnauthorized       = "Authentication required"
	MsgInvalidAuthCtx     = "Invalid authentication context"
	MsgUserNotFound       = "User not found"
	MsgInternalError      = "An unexpected error occurred"
)

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
	ErrCodeUnauthorized       = "unauthorized"
	ErrCodeUserNotFound       = "user_not_found"
	ErrCodeInternalError      = "internal_error"
)

func NewProblem(status int, code string) Problem {
	return Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
	}
}
