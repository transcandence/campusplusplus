package handler

import (
	"net/mail"
	"strings"
)

// normalizeEmail lowercases/trims the address and checks the basic RFC 5322
// format. Length is bounded by the users.email column (VARCHAR(150), but the
// API contract allows 254; we keep the contract limit and let PostgreSQL
// reject anything above its own limit as a 422).
func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return email, true
}

// normalizePhone trims the phone and enforces the users.phone_number column
// limit (VARCHAR(20)) so invalid values produce a 422 instead of a database
// error. The phone is profile data only; it is not a login identifier.
func normalizePhone(raw string) (string, bool) {
	phone := strings.TrimSpace(raw)
	if phone == "" || len(phone) > 20 {
		return "", false
	}
	return phone, true
}

func validGender(gender string) bool {
	switch gender {
	case "male", "female", "other":
		return true
	default:
		return false
	}
}

// validFullName matches the API contract (1..150 chars) and the database
// columns first_name/last_name (VARCHAR(30) each).
func validFullName(fullName string) bool {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" || len(fullName) > 150 {
		return false
	}
	for _, part := range strings.Fields(fullName) {
		if len(part) > 30 {
			return false
		}
	}
	return true
}
