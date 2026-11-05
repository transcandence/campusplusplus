package service

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/transcandence/campusplusplus/platform/go/auth"
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

// Login authenticates a user by email and password, creates an active session
// with a hashed refresh token, and returns a signed access token and raw refresh token pair.
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

	// Generate opaque refresh token and persist only its SHA-256 hash.
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

	// Issue short-lived RS256 JWT access token.
	accessToken, err := auth.GenerateAccessToken(
		int(user.ID),
		sess.FamilyID,
		int(user.CampusID),
		int(sess.ID),
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
