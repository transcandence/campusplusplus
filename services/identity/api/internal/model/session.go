package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID         int64      `db:"id"`
	UserID     int64      `db:"user_id"`
	TokenHash  []byte     `db:"token_hash"`
	IPAddress  string     `db:"ip_address"`
	UserAgent  *string    `db:"user_agent"`
	ExpiresAt  time.Time  `db:"expires_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
	FamilyID   uuid.UUID  `db:"family_id"`
	RotatedAt  *time.Time `db:"rotated_at"`
	ReplacedBy *int64     `db:"replaced_by"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

func (s *Session) Active() bool { return s.RevokedAt == nil }
