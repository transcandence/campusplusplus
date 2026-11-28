package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/transcandence/campusplusplus/services/housing/api/internal/model"
)

const testRequestID = "11111111-1111-1111-1111-111111111111"

type fakeCleaningService struct {
	called bool

	gotStudentID int64
	gotDate      *time.Time
	gotSlot      model.CleaningSlot

	result *model.CleaningRequest
	err    error
}

func (f *fakeCleaningService) CreateCleaningRequest(
	_ context.Context,
	studentID int64,
	scheduledDate *time.Time,
	slot model.CleaningSlot,
) (*model.CleaningRequest, error) {
	f.called = true
	f.gotStudentID = studentID
	f.gotSlot = slot
	if scheduledDate != nil {
		d := *scheduledDate
		f.gotDate = &d
	}
	return f.result, f.err
}

func cleaningRouter(svc CleaningService, claims *Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Header("X-Request-Id", testRequestID)
		c.Set("requestId", testRequestID)
		if claims != nil {
			c.Set("userID", claims.UserID)
			c.Set("role", claims.Role)
			c.Set("workerRole", claims.WorkerRole)
			c.Set("campusID", claims.CampusID)
			c.Set("sessionID", claims.SessionID)
		}
		c.Next()
	})
	r.POST("/housing/cleaning-requests", NewHandler(svc).CreateCleaningRequest)
	return r
}

func postCleaningRequest(t *testing.T, r http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/housing/cleaning-requests", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func sampleRequest() *model.CleaningRequest {
	return &model.CleaningRequest{
		ID:            1,
		DormUnitID:    7,
		StudentID:     42,
		StayID:        3,
		ScheduledDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		Slot:          model.CleaningSlotMorning,
		Status:        model.CleaningRequestStatusPending,
		Year:          2026,
		ISOWeek:       53,
		CreatedAt:     time.Date(2026, time.December, 31, 10, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.December, 31, 10, 0, 0, 0, time.UTC),
	}
}

func assertProblem(t *testing.T, w *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, status, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	if got := w.Header().Get("X-Request-Id"); got != testRequestID {
		t.Fatalf("X-Request-Id = %q, want %q", got, testRequestID)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding problem body: %v", err)
	}
	if body["code"] != code {
		t.Fatalf("code = %v, want %q", body["code"], code)
	}
	if body["requestId"] != testRequestID {
		t.Fatalf("requestId = %v, want %q", body["requestId"], testRequestID)
	}
	return body
}

func firstFieldError(t *testing.T, body map[string]any) (field, code string) {
	t.Helper()
	errs, ok := body["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("errors[] missing or empty: %v", body["errors"])
	}
	first, ok := errs[0].(map[string]any)
	if !ok {
		t.Fatalf("errors[0] is not an object: %v", errs[0])
	}
	field, _ = first["field"].(string)
	code, _ = first["code"].(string)
	return field, code
}

func TestCreateCleaningRequestSuccess(t *testing.T) {
	svc := &fakeCleaningService{result: sampleRequest()}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"morning","scheduledDate":"2027-01-01"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Request-Id"); got != testRequestID {
		t.Fatalf("X-Request-Id = %q, want %q", got, testRequestID)
	}
	if !svc.called {
		t.Fatal("service was not called")
	}
	if svc.gotStudentID != 42 {
		t.Errorf("student id = %d, want 42", svc.gotStudentID)
	}
	if svc.gotSlot != model.CleaningSlotMorning {
		t.Errorf("slot = %q, want morning", svc.gotSlot)
	}
	if svc.gotDate == nil || !svc.gotDate.Equal(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("scheduled date = %v, want 2027-01-01", svc.gotDate)
	}

	var body struct {
		ID            int64  `json:"id"`
		DormUnitID    int64  `json:"dormUnitId"`
		StudentID     int64  `json:"studentId"`
		StayID        int64  `json:"stayId"`
		ScheduledDate string `json:"scheduledDate"`
		Slot          string `json:"slot"`
		Status        string `json:"status"`
		Year          int    `json:"year"`
		ISOWeek       int    `json:"isoWeek"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding success body: %v", err)
	}
	if body.ID != 1 || body.DormUnitID != 7 || body.StudentID != 42 || body.StayID != 3 {
		t.Errorf("unexpected identifiers: %+v", body)
	}
	if body.ScheduledDate != "2027-01-01" {
		t.Errorf("scheduledDate = %q, want 2027-01-01", body.ScheduledDate)
	}
	if body.Slot != "morning" {
		t.Errorf("slot = %q, want morning", body.Slot)
	}
	if body.Status != "pending" {
		t.Errorf("status = %q, want pending", body.Status)
	}
	if body.Year != 2026 || body.ISOWeek != 53 {
		t.Errorf("ISO year/week = %d/%d, want 2026/53", body.Year, body.ISOWeek)
	}
}

func TestCreateCleaningRequestOmittedDatePassesNil(t *testing.T) {
	svc := &fakeCleaningService{result: sampleRequest()}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"afternoon"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	if svc.gotDate != nil {
		t.Errorf("scheduled date = %v, want nil (service must default to tomorrow)", svc.gotDate)
	}
	if svc.gotSlot != model.CleaningSlotAfternoon {
		t.Errorf("slot = %q, want afternoon", svc.gotSlot)
	}
}

func TestCreateCleaningRequestUnauthorized(t *testing.T) {
	svc := &fakeCleaningService{}
	r := cleaningRouter(svc, nil)

	w := postCleaningRequest(t, r, `{"slot":"morning"}`)
	assertProblem(t, w, http.StatusUnauthorized, "unauthorized")
	if got := w.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("WWW-Authenticate header is missing")
	}
	if svc.called {
		t.Fatal("service must not be called without authentication")
	}
}

func TestCreateCleaningRequestForbiddenRole(t *testing.T) {
	svc := &fakeCleaningService{}
	r := cleaningRouter(svc, &Claims{UserID: 2, Role: "worker", WorkerRole: "housekeeper_operator"})

	w := postCleaningRequest(t, r, `{"slot":"morning"}`)
	assertProblem(t, w, http.StatusForbidden, "forbidden")
	if svc.called {
		t.Fatal("service must not be called for a non-student role")
	}
}

func TestCreateCleaningRequestInvalidBody(t *testing.T) {
	svc := &fakeCleaningService{}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{`)
	assertProblem(t, w, http.StatusBadRequest, "invalid_body")
	if svc.called {
		t.Fatal("service must not be called for malformed JSON")
	}
}

func TestCreateCleaningRequestMissingSlot(t *testing.T) {
	svc := &fakeCleaningService{err: model.ErrInvalidSlot}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{}`)
	body := assertProblem(t, w, http.StatusUnprocessableEntity, "validation_failed")
	field, code := firstFieldError(t, body)
	if field != "slot" || code != "invalid_slot" {
		t.Errorf("first field error = %q/%q, want slot/invalid_slot", field, code)
	}
}

func TestCreateCleaningRequestNotTomorrow(t *testing.T) {
	svc := &fakeCleaningService{err: model.ErrNotTomorrow}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"morning","scheduledDate":"2026-12-31"}`)
	body := assertProblem(t, w, http.StatusUnprocessableEntity, "validation_failed")
	field, code := firstFieldError(t, body)
	if field != "scheduledDate" || code != "not_tomorrow" {
		t.Errorf("first field error = %q/%q, want scheduledDate/not_tomorrow", field, code)
	}
}

func TestCreateCleaningRequestNoActiveStay(t *testing.T) {
	svc := &fakeCleaningService{err: model.ErrNoActiveStay}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"morning"}`)
	body := assertProblem(t, w, http.StatusUnprocessableEntity, "no_active_stay")
	field, code := firstFieldError(t, body)
	if field != "stay" || code != "no_active_stay" {
		t.Errorf("first field error = %q/%q, want stay/no_active_stay", field, code)
	}
}

func TestCreateCleaningRequestQuotaConflict(t *testing.T) {
	svc := &fakeCleaningService{err: model.ErrWeeklyQuotaExceeded}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"morning"}`)
	assertProblem(t, w, http.StatusConflict, "weekly_quota_exceeded")
}

func TestCreateCleaningRequestInternalError(t *testing.T) {
	svc := &fakeCleaningService{err: errors.New("database is down")}
	r := cleaningRouter(svc, &Claims{UserID: 42, Role: "student"})

	w := postCleaningRequest(t, r, `{"slot":"morning"}`)
	assertProblem(t, w, http.StatusInternalServerError, "internal_error")
}
