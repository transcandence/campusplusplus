package service

import (
	"crypto/rsa"
	"time"
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
