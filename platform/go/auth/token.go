package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"context"
	"database/sql"
	"errors"
	"time"
	"crypto/sha256"
	"encoding/base64"
	"github.com/google/uuid"
)

type Session struct{
	ID int
	UserID int
	FamilyID uuid.UUID
	CampusID int
	SessionID int
	ExpiresAt time.Time
	RevokedAt sql.NullTime
	RotatedAt sql.NullTime
}

func RefreshSession(ctx context.Context, db *sql.DB, oldRawToken string, privateKey *rsa.PrivateKey) (string, string, error){
	oldHash := HashRefreshToken(oldRawToken)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil{
		return "", "", err
	}
	defer tx.Rollback()
	var s Session
	querySelect := `SELECT id, user_id, family_id, expires_at, revoked_at, rotated_at 
					FROM sessions s 
					JOIN users u ON s.user_id = u.id 
					WHERE s.token_hash = $1`
	err = tx.QueryRowContext(ctx, querySelect, oldHash).Scan(
		&s.ID, &s.UserID, &s.FamilyID, &s.ExpiresAt, &s.RevokedAt, &s.RotatedAt, &s.CampusID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows){
			return "", "", errors.New("invalid_refresh: token unknown")
		}
		return "", "", err
	}
	if s.RevokedAt.Valid || time.Now().After(s.ExpiresAt) {
		return "", "", errors.New("invalid_refresh : expired or revoked session")
	}
	if s.RotatedAt.Valid {
		revokeFamilyQuery := `UPDATE sessions SET revoked_at = NOW() WHERE family_id = $1`
		_, _ = tx.ExecContext(ctx, revokeFamilyQuery, s.FamilyID)
		_ = tx.Commit()
		return "", "", errors.New("security_alert: token reuse detected, entire family revoked")
	}
	newRawToken, err := GenerateOpaqueRefreshToken()
	if err != nil{
		return "", "", err
	}
	newHash := HashRefreshToken(newRawToken)
	var newSessionID int
	queryInsert := `
					INSERT INTO sessions (user_id, token_hash, ip_adress, user_agent, expires_at, family_id)
					VALUES ($1, $2, '127.0.0.1', 'API-Client', $3, $4)
					RETURNING id`
	newExpiresAT := time.Now().Add(30 * 24 * time.Hour)
	err = tx.QueryRowContext(ctx, queryInsert, s.UserID, newHash, newExpiresAT, s.FamilyID).Scan(&newSessionID)
	if err != nil {
		return "", "", err
	}
	queryUpdate := `
					UPDATE sessions
					SET revoked_at = NOW(), rotated_at = NOW(), replaced_by = $1
					WHERE id = $2`
	_, err = tx.ExecContext(ctx, queryUpdate, newSessionID, s.ID)
	if err != nil{
		return "", "", err
	}
	if err := tx.Commit(); err != nil{
		return "", "", err
	}
	newAccessToken, err := GenerateAccessToken(s.UserID, s.FamilyID, s.CampusID, s.SessionID, privateKey)
	if err != nil {
		return "", "", err
	}
	return newAccessToken, newRawToken, nil 
}

func GenerateOpaqueRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashRefreshToken(rawToken string) []byte {
	hash := sha256.Sum256([]byte(rawToken))
	return hash[:] 
}