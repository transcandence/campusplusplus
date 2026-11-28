package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// FieldError is the internal shape used by handlers to build the RFC 9457
// `errors[]` array required by the contract's ValidationProblem.
type FieldError struct {
	Field   string
	Code    string
	Message string
}

// RespondProblem writes an RFC 9457 Problem Details response.
func RespondProblem(c *gin.Context, status int, code, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, Problem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Code:      &code,
		Detail:    &detail,
		RequestId: requestID(c),
	})
}

// RespondValidationProblem writes a ValidationProblem (422) with the required
// `errors[]` array. Use it for every 422; a plain Problem does not satisfy the
// housing contract for that status.
func RespondValidationProblem(c *gin.Context, status int, code, detail string, fields []FieldError) {
	p := ValidationProblem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Code:      &code,
		Detail:    &detail,
		RequestId: requestID(c),
		Errors: make([]struct {
			Code    string  `json:"code"`
			Field   string  `json:"field"`
			Message *string `json:"message,omitempty"`
		}, 0, len(fields)),
	}

	for _, f := range fields {
		var message *string
		if f.Message != "" {
			m := f.Message
			message = &m
		}
		p.Errors = append(p.Errors, struct {
			Code    string  `json:"code"`
			Field   string  `json:"field"`
			Message *string `json:"message,omitempty"`
		}{
			Code:    f.Code,
			Field:   f.Field,
			Message: message,
		})
	}

	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, p)
}

func requestID(c *gin.Context) *openapi_types.UUID {
	raw := c.GetString("requestId")
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func notImplemented(c *gin.Context) {
	RespondProblem(c, http.StatusNotImplemented, "not_implemented", "Not implemented yet")
}
