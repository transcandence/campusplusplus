package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/service"
)

type fakeUserRepository struct {
	user        *model.User
	me          *model.UserMe
	createErr   error
	updateErr   error
	updateInput model.UpdateMeInput
}

func (f *fakeUserRepository) CheckEmailOrPhoneExists(context.Context, *string, *string) (bool, error) {
	return false, nil
}

func (f *fakeUserRepository) CreateUserWithOutbox(context.Context, model.User, []byte) (int64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	return 42, nil
}

func (f *fakeUserRepository) FindByEmail(context.Context, string) (*model.User, error) {
	if f.user == nil {
		return nil, model.ErrUserNotFound
	}
	return f.user, nil
}

func (f *fakeUserRepository) FindByID(context.Context, int64) (*model.User, error) {
	if f.user == nil {
		return nil, model.ErrUserNotFound
	}
	return f.user, nil
}

func (f *fakeUserRepository) FindMeByID(_ context.Context, userID int64) (*model.UserMe, error) {
	if f.me != nil {
		return f.me, nil
	}
	return &model.UserMe{ID: userID, Status: model.StatusActive}, nil
}

func (f *fakeUserRepository) FindCampusIDByCode(context.Context, model.CampusCode) (int64, error) {
	return 1, nil
}

func (f *fakeUserRepository) UpdateMe(_ context.Context, _ int64, input model.UpdateMeInput) (*model.UserMe, error) {
	f.updateInput = input
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.FindMeByID(context.Background(), 42)
}

func newAuthRouter(userRepo *fakeUserRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	authService := service.NewAuthService(userRepo, &logoutSessionRepository{}, nil, time.Minute, time.Hour, 10)
	authHandler := NewAuthHandler(authService, nil)

	router := gin.New()
	router.POST("/auth/login", authHandler.Login)
	router.PUT("/me", func(c *gin.Context) {
		c.Set("userID", int64(42))
		c.Next()
	}, authHandler.UpdateMe)
	return router
}

func performJSON(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestLoginSuspendedAccountReturns403(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	email := "suspended@example.com"
	repo := &fakeUserRepository{user: &model.User{
		ID:           7,
		Email:        &email,
		PasswordHash: string(hash),
		Status:       model.StatusSuspended,
		Role:         model.RoleStudent,
	}}
	router := newAuthRouter(repo)

	recorder := performJSON(router, http.MethodPost, "/auth/login", `{"email":"suspended@example.com","password":"Password123"}`)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeAccountSuspended {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeAccountSuspended)
	}
}

func TestUpdateMeAcceptsPartialFields(t *testing.T) {
	repo := &fakeUserRepository{me: &model.UserMe{ID: 42, Email: "new@example.com", Status: model.StatusActive}}
	router := newAuthRouter(repo)

	recorder := performJSON(router, http.MethodPut, "/me", `{"email":"new@example.com"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if repo.updateInput.Email == nil || *repo.updateInput.Email != "new@example.com" {
		t.Errorf("email input = %v, want new@example.com", repo.updateInput.Email)
	}
	if repo.updateInput.FullName != nil {
		t.Errorf("fullName input = %v, want nil", repo.updateInput.FullName)
	}
	if repo.updateInput.Gender != nil {
		t.Errorf("gender input = %v, want nil", repo.updateInput.Gender)
	}
}

func TestUpdateMeWithoutFullNameDoesNotRequireIt(t *testing.T) {
	repo := &fakeUserRepository{me: &model.UserMe{ID: 42, Status: model.StatusActive}}
	router := newAuthRouter(repo)

	recorder := performJSON(router, http.MethodPut, "/me", `{"gender":"female"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if repo.updateInput.Gender == nil || *repo.updateInput.Gender != "female" {
		t.Errorf("gender input = %v, want female", repo.updateInput.Gender)
	}
	if repo.updateInput.FullName != nil {
		t.Errorf("fullName input = %v, want nil", repo.updateInput.FullName)
	}
}

func TestUpdateMeRejectsInvalidGenderWith422(t *testing.T) {
	repo := &fakeUserRepository{me: &model.UserMe{ID: 42, Status: model.StatusActive}}
	router := newAuthRouter(repo)

	recorder := performJSON(router, http.MethodPut, "/me", `{"gender":"invalid"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeValidationFailed {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeValidationFailed)
	}
}

func TestUpdateMeDuplicateContactReturns409(t *testing.T) {
	repo := &fakeUserRepository{updateErr: model.ErrUserAlreadyExists}
	router := newAuthRouter(repo)

	recorder := performJSON(router, http.MethodPut, "/me", `{"phone":"+212600000001"}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeUserAlreadyExists {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeUserAlreadyExists)
	}
}
