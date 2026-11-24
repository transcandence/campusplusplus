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
	"github.com/transcandence/campusplusplus/services/identity/api/internal/repository"
)

type AuthService struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	privateKey  *rsa.PrivateKey
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	privateKey *rsa.PrivateKey,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		privateKey:  privateKey,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
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
	if err := s.sessionRepo.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	accessToken, err := auth.GenerateAccessToken(
		user.ID,
		sess.ID,
		user.CampusID,
		string(user.Role),
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
	if lastName == ""{
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
        Status:       model.StatusPendingVerification,
        CampusID:     input.CampusID,
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
		if errors.Is(err, model.ErrSessionNotFound){
			return nil, model.ErrInvalidRefresh
		}
		return nil, fmt.Errorf("finding session by token hash: %w", err)
	}
	if session.RevokedAt != nil && session.RotatedAt != nil {
		_ = s.sessionRepo.RevokeFamily(ctx, session.FamilyID)
		return nil, model.ErrInvalidRefresh 
	}
	if session.RevokedAt != nil || time.Now().UTC().After(session.ExpiresAt) {
		return nil, model.ErrInvalidRefresh
	}
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil{
		if errors.Is(err, model.ErrUserNotFound){
			return nil, model.ErrInvalidRefresh
		}
		return nil, fmt.Errorf("finding user for session: %w", err)
	}
	switch user.Status{
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
		UserID: user.ID,
		TokenHash: newNodeHash,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(s.refreshTTL),
		FamilyID: session.FamilyID,
	}
	if err := s.sessionRepo.Rotate(ctx, session.ID, newSession); err != nil{
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
		newSession.FamilyID,
		s.accessTTL,
		s.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}
	return &TokenPair{
		TokenType: "Bearer",
		AccessToken: accessToken,
		ExpiresIn: int(s.accessTTL.Seconds()),
		RefreshToken: newRawRefreshToken,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error{
	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	session, err := s.sessionRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, model.ErrSessionNotFound) {
			return nil
		}
		return fmt.Errorf("finding session for logout: %w", err)
	}
	if err := s.sessionRepo.RevokeByID(ctx, session.ID); err != nil{
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID int64) error{
	if err := s.sessionRepo.RevokeAllByUserID(ctx, userID); err != nil{
		return fmt.Errorf("revoking all sessions for user: %w", err)
	}
	return nil
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
func (s *AuthService) UpdateMe(ctx context.Context, userID int64, fullName string, gender *string) (*model.UserMe, error) {
	firstName, lastName := splitFullName(fullName)
	return s.userRepo.UpdateMe(ctx, userID, firstName, lastName, gender)
}
