package auth

import(
	"crypto/rsa"
	"fmt"
	"os"
	"github.com/golang-jwt/jwt/v5"
)

func LoadPrivateKey(path string) (*rsa.PrivateKey, error){
	data, err := os.ReadFile(path)
	if (err != nil){
		return nil, fmt.Errorf("Failed to read private key file: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse private key: %w", err)
	}
	return key, nil
}

func LoadPublicKey(path string) (*rsa.PublicKey, error){
	data, err := os.ReadFile(path)
	if err != nil{
		return nil, fmt.Errorf("Failed to read public key file: %w", err)
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse public key: %w", err)
	}
	return key, nil
}
