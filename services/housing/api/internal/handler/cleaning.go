package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

// CreateCleaningRequest implements POST /housing/cleaning-requests.
//
// Authorization is checked before reading the body: only an authenticated
// student can book a cleaning for their own active stay.
func (h *Handler) CreateCleaningRequest(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		c.Header("WWW-Authenticate", `Bearer realm="campus-api"`)
		RespondProblem(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	if claims.Role != "student" {
		RespondProblem(c, http.StatusForbidden, "forbidden", "Only students can create cleaning requests")
		return
	}

	var req CleaningRequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondProblem(c, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	var scheduled *time.Time
	if req.ScheduledDate != nil {
		t := req.ScheduledDate.Time
		scheduled = &t
	}

	result, err := h.cleaning.CreateCleaningRequest(
		c.Request.Context(),
		claims.UserID,
		scheduled,
		model.CleaningSlot(req.Slot),
	)
	if err != nil {
		respondCreateCleaningError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toCleaningRequestResponse(result))
}

func respondCreateCleaningError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidSlot):
		RespondValidationProblem(c, http.StatusUnprocessableEntity, "validation_failed",
			"Invalid cleaning slot", []FieldError{{
				Field:   "slot",
				Code:    "invalid_slot",
				Message: "slot must be morning or afternoon",
			}})

	case errors.Is(err, model.ErrNotTomorrow):
		RespondValidationProblem(c, http.StatusUnprocessableEntity, "validation_failed",
			"Scheduled date must be tomorrow", []FieldError{{
				Field:   "scheduledDate",
				Code:    "not_tomorrow",
				Message: "scheduledDate must be exactly tomorrow",
			}})

	case errors.Is(err, model.ErrNoActiveStay):
		RespondValidationProblem(c, http.StatusUnprocessableEntity, "no_active_stay",
			"The caller has no active stay", []FieldError{{
				Field:   "stay",
				Code:    "no_active_stay",
				Message: "an active stay is required to book a cleaning",
			}})

	case errors.Is(err, model.ErrWeeklyQuotaExceeded):
		RespondProblem(c, http.StatusConflict, "weekly_quota_exceeded",
			"This unit already has a cleaning request for that ISO week")

	case errors.Is(err, model.ErrInvalidInput):
		RespondValidationProblem(c, http.StatusUnprocessableEntity, "validation_failed",
			"Invalid input", []FieldError{{
				Field: "_",
				Code:  "invalid_input",
			}})

	default:
		RespondProblem(c, http.StatusInternalServerError, "internal_error",
			"An unexpected error occurred")
	}
}

func toCleaningRequestResponse(r *model.CleaningRequest) CleaningRequest {
	return CleaningRequest{
		Id:            r.ID,
		DormUnitId:    r.DormUnitID,
		StudentId:     r.StudentID,
		StayId:        r.StayID,
		ScheduledDate: openapi_types.Date{Time: r.ScheduledDate},
		Slot:          CleaningSlot(r.Slot),
		Status:        CleaningRequestStatus(r.Status),
		Year:          r.Year,
		IsoWeek:       r.ISOWeek,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}
