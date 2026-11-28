package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateAccessTokenUsesRS256AndNumericCampusID(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	tokenString, err := GenerateAccessToken(
		42,
		7,
		3,
		"student",
		uuid.New(),
		time.Minute,
		privateKey,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	parsed, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parsing token: %v", err)
	}
	if alg, _ := parsed.Header["alg"].(string); alg != "RS256" {
		t.Errorf("alg = %q, want RS256", alg)
	}

	claims, err := VerifyAccessToken(tokenString, &privateKey.PublicKey)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("user_id = %d, want 42", claims.UserID)
	}
	if claims.SessionID != 7 {
		t.Errorf("sid = %d, want 7", claims.SessionID)
	}
	if claims.CampusID != 3 {
		t.Errorf("campus_id = %d, want numeric 3", claims.CampusID)
	}
	if claims.Issuer != "campus-identity" {
		t.Errorf("iss = %q, want campus-identity", claims.Issuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "campus-api" {
		t.Errorf("aud = %v, want [campus-api]", claims.Audience)
	}
}
