// Package service implements the core identity business logic, including
// user authentication, session management, and JWT token issuance.
package service

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	// "os/user"
	"strings"
	"time"

	// ... tes imports existants ...
	"encoding/json"

	"github.com/google/uuid"
	"github.com/transcandence/campusplusplus/platform/go/auth"
	"golang.org/x/crypto/bcrypt"

	// "github.com/transcandence/campusplusplus/services/identity/api/internal/handler"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
)

// UserRepository is the subset of user persistence used by the auth service.
// It is an interface so the service can be tested without a database.
type UserRepository interface {
	CheckEmailOrPhoneExists(ctx context.Context, email, phoneNumber *string) (bool, error)
	CreateUserWithOutbox(ctx context.Context, user model.User, eventPayload []byte) (int64, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindMeByID(ctx context.Context, userID int64) (*model.UserMe, error)
	FindCampusIDByCode(ctx context.Context, code model.CampusCode) (int64, error)
	UpdateMe(ctx context.Context, userID int64, input model.UpdateMeInput) (*model.UserMe, error)
}

// SessionRepository is the subset of session persistence used by the auth service.
type SessionRepository interface {
	Create(ctx context.Context, s *model.Session) error
	CreateWithLimit(ctx context.Context, s *model.Session, maxActive int) error
	FindByTokenHash(ctx context.Context, tokenHash []byte) (*model.Session, error)
	Rotate(ctx context.Context, oldSessionID int64, newSession *model.Session) error
	RevokeByID(ctx context.Context, sessionID int64) error
	RevokeByIDForUser(ctx context.Context, userID, sessionID int64) (bool, error)
	RevokeAllByUserID(ctx context.Context, userID int64) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
	ListActiveByUserID(ctx context.Context, userID int64) ([]model.SessionSummary, error)
}

type AuthService struct {
	userRepo          UserRepository
	sessionRepo       SessionRepository
	privateKey        *rsa.PrivateKey
	accessTTL         time.Duration
	refreshTTL        time.Duration
	maxActiveSessions int
}

func NewAuthService(
	userRepo UserRepository,
	sessionRepo SessionRepository,
	privateKey *rsa.PrivateKey,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	maxActiveSessions int,
) *AuthService {
	return &AuthService{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		privateKey:        privateKey,
		accessTTL:         accessTTL,
		refreshTTL:        refreshTTL,
		maxActiveSessions: maxActiveSessions,
	}
}

type TokenPair struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// type UserRepo interface {
// CreateUserWithOutbox(ctx context.Context, user model.User, eventPayload []byte) error
// CheckEmailOrPhoneExists(ctx context.Context, email, phone_number *string) (bool, error)
// }

func (s *AuthService) Login(
	ctx context.Context,
	email string,
	password string,
	ipAddress string,
	userAgent *string,
) (*TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("finding user: %w", err)
	}

	// Verify password against stored bcrypt hash.
	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, model.ErrInvalidCredentials
	}

	// Guard account status before issuing credentials.
	switch user.Status {
	case model.StatusSuspended:
		return nil, model.ErrAccountSuspended
	case model.StatusPendingVerification:
		return nil, model.ErrAccountPending
	case model.StatusActive:
		// Proceed!
	default:
		return nil, model.ErrInvalidCredentials
	}

	rawRefreshToken, err := auth.GenerateOpaqueRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}
	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	now := time.Now().UTC()
	sess := &model.Session{
		UserID:    user.ID,
		TokenHash: tokenHash,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(s.refreshTTL),
		FamilyID:  uuid.New(),
	}
	if err := s.sessionRepo.CreateWithLimit(ctx, sess, s.maxActiveSessions); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	accessToken, err := auth.GenerateAccessToken(
		user.ID,
		sess.ID,
		user.CampusID,
		string(user.Role),
		user.WorkerRoleOrZero(),
		sess.FamilyID,
		s.accessTTL,
		s.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	return &TokenPair{
		TokenType:    "Bearer",
		AccessToken:  accessToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: rawRefreshToken,
	}, nil
}

func splitFullName(fullName string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(fullName))
	if len(parts) == 0 {
		return "", ""
	}
	firstName := parts[0]
	lastName := strings.Join(parts[1:], " ")
	if lastName == "" {
		lastName = firstName
	}
	return firstName, lastName
}

func (s *AuthService) Register(ctx context.Context, input model.RegisterInput) (*model.UserMe, error) {
	role := model.Role(strings.ToLower(strings.TrimSpace(input.UserType)))
	if !model.IsSelfRegisterRole(role) {
		return nil, model.ErrInvalidRole
	}
	var email, phone *string
	if input.Email != nil {
		if e := strings.ToLower(strings.TrimSpace(*input.Email)); e != "" {
			email = &e
		}
	}
	if input.Phone != nil {
		if p := strings.TrimSpace(*input.Phone); p != "" {
			phone = &p
		}
	}
	// Email is the only login identifier. Enforce it here as well as in the
	// HTTP handler so the rule cannot be bypassed by another caller.
	if email == nil || *email == "" {
		return nil, model.ErrEmailRequired
	}
	exists, err := s.userRepo.CheckEmailOrPhoneExists(ctx, email, phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, model.ErrUserAlreadyExists
	}

	if input.Password == nil {
		return nil, errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	firstName, lastName := splitFullName(input.FullName)
	affiliation := model.Affiliation(input.Affiliation)
	if affiliation == "" {
		affiliation = model.AffiliationUM6P
	}
	newUser := model.User{
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		PhoneNumber:  phone,
		Gender:       input.Gender,
		PasswordHash: string(hash),
		Affiliation:  affiliation,
		Role:         role,
		// There is no email-verification flow yet: new accounts are usable
		// immediately so the register -> login path can be tested end to end.
		Status:   model.StatusActive,
		CampusID: input.CampusID,
	}
	eventPayload, err := json.Marshal(map[string]any{
		"fullName": input.FullName,
		"email":    email,
		"phone":    phone,
		"userType": string(role),
	})
	if err != nil {
		return nil, err
	}
	userID, err := s.userRepo.CreateUserWithOutbox(ctx, newUser, eventPayload)
	if err != nil {
		return nil, err
	}
	return s.userRepo.FindMeByID(ctx, userID)
}

func (s *AuthService) Refresh(
	ctx context.Context,
	rawRefreshToken string,
	ipAddress string,
	userAgent *string,
) (*TokenPair, error) {
	TokenHash := auth.HashRefreshToken(rawRefreshToken)
	session, err := s.sessionRepo.FindByTokenHash(ctx, TokenHash)
	if err != nil {
		if errors.Is(err, model.ErrSessionNotFound) {
			return nil, model.ErrInvalidRefresh
		}
		return nil, fmt.Errorf("finding session by token hash: %w", err)
	}
	if session.RevokedAt != nil && session.RotatedAt != nil {
		// Reuse of a rotated token means the family is compromised. If we
		// cannot revoke it, surface a 500 instead of pretending the threat
		// was handled.
		if err := s.sessionRepo.RevokeFamily(ctx, session.FamilyID); err != nil {
			return nil, fmt.Errorf("revoking reused session family: %w", err)
		}
		return nil, model.ErrInvalidRefresh
	}
	if session.RevokedAt != nil || time.Now().UTC().After(session.ExpiresAt) {
		return nil, model.ErrInvalidRefresh
	}
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrInvalidRefresh
		}
		return nil, fmt.Errorf("finding user for session: %w", err)
	}
	switch user.Status {
	case model.StatusActive:
	case model.StatusSuspended:
		return nil, model.ErrAccountSuspended
	default:
		return nil, model.ErrInvalidRefresh
	}
	newRawRefreshToken, err := auth.GenerateOpaqueRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating new refresh token: %w", err)
	}
	newNodeHash := auth.HashRefreshToken(newRawRefreshToken)
	now := time.Now().UTC()
	newSession := &model.Session{
		UserID:    user.ID,
		TokenHash: newNodeHash,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(s.refreshTTL),
		FamilyID:  session.FamilyID,
	}
	if err := s.sessionRepo.Rotate(ctx, session.ID, newSession); err != nil {
		if errors.Is(err, model.ErrSessionNotFound) {
			return nil, model.ErrInvalidRefresh
		}
		return nil, fmt.Errorf("rotating session in repo: %w", err)
	}
	accessToken, err := auth.GenerateAccessToken(
		user.ID,
		newSession.ID,
		user.CampusID,
		string(user.Role),
		user.WorkerRoleOrZero(),
		newSession.FamilyID,
		s.accessTTL,
		s.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}
	return &TokenPair{
		TokenType:    "Bearer",
		AccessToken:  accessToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: newRawRefreshToken,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	session, err := s.sessionRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, model.ErrSessionNotFound) {
			return nil
		}
		return fmt.Errorf("finding session for logout: %w", err)
	}
	if err := s.sessionRepo.RevokeByID(ctx, session.ID); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID int64) error {
	if err := s.sessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("revoking all sessions for user: %w", err)
	}
	return nil
}

// ListSessions returns the safe session summaries for the authenticated user.
func (s *AuthService) ListSessions(ctx context.Context, userID int64) ([]model.SessionSummary, error) {
	sessions, err := s.sessionRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing sessions for user: %w", err)
	}
	return sessions, nil
}

// RevokeSession revokes one session owned by userID. It returns false when
// the session does not exist, is already revoked, or belongs to someone else.
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID int64) (bool, error) {
	revoked, err := s.sessionRepo.RevokeByIDForUser(ctx, userID, sessionID)
	if err != nil {
		return false, fmt.Errorf("revoking session for user: %w", err)
	}
	return revoked, nil
}

func (s *AuthService) GetMe(ctx context.Context, userID int64) (*model.UserMe, error) {
	user, err := s.userRepo.FindMeByID(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("finding user profile by id %w", err)
	}
	return user, nil
}

// ResolveCampusID validates a campus code and returns its database ID.
func (s *AuthService) ResolveCampusID(ctx context.Context, code string) (int64, error) {
	campusCode := model.CampusCode(strings.ToLower(strings.TrimSpace(code)))
	if !campusCode.Valid() {
		return 0, fmt.Errorf("invalid campus code: %q", code)
	}
	return s.userRepo.FindCampusIDByCode(ctx, campusCode)
}

// UpdateMe updates the authenticated user's editable profile fields.
// Only the fields present in input are changed.
func (s *AuthService) UpdateMe(ctx context.Context, userID int64, input model.UpdateMeInput) (*model.UserMe, error) {
	if input.FullName != nil {
		firstName, lastName := splitFullName(*input.FullName)
		input.FirstName = &firstName
		input.LastName = &lastName
	}
	return s.userRepo.UpdateMe(ctx, userID, input)
}
