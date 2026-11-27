package model

import (
	"time"

	"github.com/google/uuid"
)

// SessionSummary is the safe, client-facing view of a session. It never
// contains token hashes, family ids, or rotation metadata.
type SessionSummary struct {
	ID         int64      `json:"id"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	IPAddress  string     `json:"ipAddress,omitempty"`
	UserAgent  *string    `json:"userAgent,omitempty"`
	Current    bool       `json:"current"`
}

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
