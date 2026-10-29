package auth

import (
	"crypto/rsa"
	"errors"
	"time"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct
{
	UserID int `json:"sub"`
	Role string  `json:"role"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(UserID int, role string, privateKey *rsa.PrivateKey) (string, error){
	claims := Claims {
		UserID: UserID,
		Role: role,
		RegisteredClaims : jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer: "campus-auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func VerifyAccessToken(tokenString string, publicKey *rsa.PublicKey) (*Claims, error){
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("Invalid token claims")
	}
	return claims, nil
}
