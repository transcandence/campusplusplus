package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

// CleaningService is the handler's view of the cleaning use cases. Keeping it
// narrow lets the handler tests use a fake.
type CleaningService interface {
	CreateCleaningRequest(
		ctx context.Context,
		studentID int64,
		scheduledDate *time.Time,
		slot model.CleaningSlot,
	) (*model.CleaningRequest, error)
}

// Handler implements the generated ServerInterface. Only
// CreateCleaningRequest is wired in this step; the other endpoints answer 501
// until their PR lands.
type Handler struct {
	cleaning CleaningService
}

func NewHandler(cleaning CleaningService) *Handler {
	return &Handler{cleaning: cleaning}
}

// Compile-time check that every generated endpoint is implemented.
var _ ServerInterface = (*Handler)(nil)

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Stubs for endpoints outside this PR --------------------------------

func (h *Handler) ListCleaningTasks(c *gin.Context, params ListCleaningTasksParams) {
	notImplemented(c)
}

func (h *Handler) CompleteCleaningTask(c *gin.Context, taskId int64) {
	notImplemented(c)
}

func (h *Handler) StartCleaningTask(c *gin.Context, taskId int64) {
	notImplemented(c)
}

func (h *Handler) ListDormUnits(c *gin.Context, params ListDormUnitsParams) {
	notImplemented(c)
}

func (h *Handler) AssignHousekeeper(c *gin.Context) {
	notImplemented(c)
}

func (h *Handler) ListMyCleaningRequests(c *gin.Context) {
	notImplemented(c)
}

func (h *Handler) ListMyShifts(c *gin.Context, params ListMyShiftsParams) {
	notImplemented(c)
}

func (h *Handler) GetMyStay(c *gin.Context) {
	notImplemented(c)
}

func (h *Handler) ListResidences(c *gin.Context, params ListResidencesParams) {
	notImplemented(c)
}

func (h *Handler) ListStays(c *gin.Context, params ListStaysParams) {
	notImplemented(c)
}

func (h *Handler) CreateStay(c *gin.Context) {
	notImplemented(c)
}

func (h *Handler) GetStay(c *gin.Context, stayId int64) {
	notImplemented(c)
}

func (h *Handler) CancelStay(c *gin.Context, stayId int64) {
	notImplemented(c)
}

func (h *Handler) CheckInStay(c *gin.Context, stayId int64) {
	notImplemented(c)
}

func (h *Handler) CheckOutStay(c *gin.Context, stayId int64) {
	notImplemented(c)
}
