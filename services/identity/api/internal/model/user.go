package model

import "time"

type Affiliation string

const (
	Affiliation1337  Affiliation = "1337"
	AffiliationABS   Affiliation = "ABS"
	AffiliationFGSES Affiliation = "FGSES"
	AffiliationSHBM  Affiliation = "SHBM"
	AffiliationSAS   Affiliation = "SAS"
	AffiliationCC    Affiliation = "CC"
	AffiliationAEM   Affiliation = "AEM"
	AffiliationUM6P  Affiliation = "um6p"
)

type Role string

const (
	RoleStudent Role = "student"
	RoleStaff   Role = "staff"
	RoleAdmin   Role = "admin"
	RoleGuest   Role = "guest"
	RoleWorker  Role = "worker"
)

type Status string

const (
	StatusPendingVerification Status = "pending_verification"
	StatusActive              Status = "active"
	StatusSuspended           Status = "suspended"
)

type RegisterInput struct {
    FullName string
    Email    *string
    Phone    *string
    Password *string
    UserType string
    Gender   *string
	CampusID int64
	Affiliation Affiliation
}

type User struct {
	ID           int64       `db:"id"`
	FirstName    string      `db:"first_name"`
	LastName     string      `db:"last_name"`
	Email        *string      `db:"email"`
	PhoneNumber  *string      `db:"phone_number"`
	Birthday     *time.Time  `db:"birthday"`
	Affiliation  Affiliation `db:"affiliation"`
	Role         Role        `db:"role"`
	Gender       *string     `db:"gender"`
	PasswordHash string      `db:"password_hash"`
	Nationality  *string     `db:"nationality"`
	Status       Status      `db:"status"`
	CampusID     int64       `db:"campus_id"`
	CreatedAt    time.Time   `db:"created_at"`
	UpdatedAt    time.Time   `db:"updated_at"`
}

type UserMe struct {
	ID          int64       `json:"id"`
	Email       string      `json:"email"`
	FirstName   string      `json:"firstName"`
	LastName    string      `json:"lastName"`
	Affiliation Affiliation `json:"affiliation"`
	Role        Role        `json:"role"`
	Status      Status      `json:"status"`
	Campus      CampusCode  `json:"campus"`
	CreatedAt   time.Time   `json:"createdAt"`
}

func IsSelfRegisterRole(r Role) bool {
    switch r {
    case RoleStudent, RoleGuest:
        return true
    }
    return false
}

func (u *User) ToMe(campus CampusCode) UserMe {
	email := ""
    if u.Email != nil {
        email = *u.Email
    }
    return UserMe{
        ID: u.ID, Email: email, FirstName: u.FirstName, LastName: u.LastName,
        Affiliation: u.Affiliation, Role: u.Role, Status: u.Status,
        Campus: campus, CreatedAt: u.CreatedAt,
    }
}
