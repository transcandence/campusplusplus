// Package service implements the core identity business logic, including
// user authentication, session management, and JWT token issuance.
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
		return nil, fmt.Errorf("finding user for session: %w", err)
	}
	if user.Status != model.StatusActive {
		return nil, model.ErrAccountSuspended
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
