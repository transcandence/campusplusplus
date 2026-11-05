package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID    int64     `json:"user_id"`
	SessionID int64     `json:"sid"`
	CampusID  int64     `json:"campus_id"`
	Role      string    `json:"role"`
	FamilyID  uuid.UUID `json:"family_id,omitempty"`
	jwt.RegisteredClaims
}

// GenerateAccessToken signs an RS256 JWT access token with standard RFC 7519 and campus claims.
func GenerateAccessToken(
	userID int64,
	sessionID int64,
	campusID int64,
	role string,
	familyID uuid.UUID,
	ttl time.Duration,
	privateKey *rsa.PrivateKey,
) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:    userID,
		SessionID: sessionID,
		CampusID:  campusID,
		Role:      role,
		FamilyID:  familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    "campus-identity",
			Audience:  jwt.ClaimStrings{"campus-api"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return signed, nil
}

// VerifyAccessToken parses and validates an RS256 access token with the public key.
func VerifyAccessToken(tokenString string, publicKey *rsa.PublicKey) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
