package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

type fakeUserRepository struct {
	created model.User
}

func (f *fakeUserRepository) CheckEmailOrPhoneExists(context.Context, *string, *string) (bool, error) {
	return false, nil
}

func (f *fakeUserRepository) CreateUserWithOutbox(_ context.Context, user model.User, _ []byte) (int64, error) {
	f.created = user
	return 42, nil
}

func (f *fakeUserRepository) FindByEmail(context.Context, string) (*model.User, error) {
	return nil, model.ErrUserNotFound
}

func (f *fakeUserRepository) FindByID(context.Context, int64) (*model.User, error) {
	return nil, model.ErrUserNotFound
}

func (f *fakeUserRepository) FindMeByID(_ context.Context, userID int64) (*model.UserMe, error) {
	return &model.UserMe{ID: userID, Status: f.created.Status}, nil
}

func (f *fakeUserRepository) FindCampusIDByCode(context.Context, model.CampusCode) (int64, error) {
	return 1, nil
}

func (f *fakeUserRepository) UpdateMe(context.Context, int64, model.UpdateMeInput) (*model.UserMe, error) {
	return nil, nil
}

type fakeSessionRepository struct {
	session         *model.Session
	findErr         error
	revokeFamilyErr error
}

func (fakeSessionRepository) Create(context.Context, *model.Session) error { return nil }

func (fakeSessionRepository) CreateWithLimit(context.Context, *model.Session, int) error {
	return nil
}

func (f fakeSessionRepository) FindByTokenHash(context.Context, []byte) (*model.Session, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.session != nil {
		return f.session, nil
	}
	return nil, model.ErrSessionNotFound
}

func (fakeSessionRepository) Rotate(context.Context, int64, *model.Session) error { return nil }

func (fakeSessionRepository) RevokeByID(context.Context, int64) error { return nil }

func (fakeSessionRepository) RevokeByIDForUser(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (fakeSessionRepository) RevokeAllByUserID(context.Context, int64) error { return nil }

func (f fakeSessionRepository) RevokeFamily(context.Context, uuid.UUID) error {
	return f.revokeFamilyErr
}

func (fakeSessionRepository) ListActiveByUserID(context.Context, int64) ([]model.SessionSummary, error) {
	return nil, nil
}

func newTestAuthService(userRepo UserRepository) *AuthService {
	return NewAuthService(userRepo, fakeSessionRepository{}, nil, time.Minute, time.Hour, 10)
}

func TestRegisterCreatesActiveUser(t *testing.T) {
	repo := &fakeUserRepository{}
	svc := newTestAuthService(repo)

	email := "sara.benali@campus.ma"
	password := "Sara@benali1234"
	me, err := svc.Register(context.Background(), model.RegisterInput{
		FullName: "Sara Benali",
		Email:    &email,
		Password: &password,
		UserType: "student",
		CampusID: 1,
	})
	if err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}

	if repo.created.Status != model.StatusActive {
		t.Errorf("created user status = %q, want %q", repo.created.Status, model.StatusActive)
	}
	if me == nil || me.Status != model.StatusActive {
		t.Errorf("returned profile status = %v, want %q", me, model.StatusActive)
	}
	if repo.created.Email == nil || *repo.created.Email != email {
		t.Errorf("created user email = %v, want %q", repo.created.Email, email)
	}
}

func TestRegisterRequiresEmail(t *testing.T) {
	svc := newTestAuthService(&fakeUserRepository{})

	password := "Sara@benali1234"
	_, err := svc.Register(context.Background(), model.RegisterInput{
		FullName: "Sara Benali",
		Password: &password,
		UserType: "student",
		CampusID: 1,
	})
	if !errors.Is(err, model.ErrEmailRequired) {
		t.Fatalf("Register error = %v, want %v", err, model.ErrEmailRequired)
	}
}

func TestRefreshReusePropagatesFamilyRevokeError(t *testing.T) {
	now := time.Now().UTC()
	sessionRepo := fakeSessionRepository{
		session: &model.Session{
			ID:        1,
			FamilyID:  uuid.New(),
			RevokedAt: &now,
			RotatedAt: &now,
		},
		revokeFamilyErr: errors.New("database unavailable"),
	}
	svc := NewAuthService(&fakeUserRepository{}, sessionRepo, nil, time.Minute, time.Hour, 10)

	_, err := svc.Refresh(context.Background(), "reused-token", "127.0.0.1", nil)
	if err == nil || !strings.Contains(err.Error(), "revoking reused session family") {
		t.Fatalf("Refresh error = %v, want wrapped family revocation error", err)
	}
}
