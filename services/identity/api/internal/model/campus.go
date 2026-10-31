package model

import "time"

type Campus struct {
	ID        int64      `json:"id"        db:"id"`
	Code      CampusCode `json:"code"      db:"code"`
	Name      string     `json:"name"      db:"name"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time  `json:"updatedAt" db:"updated_at"`
}

type CampusCode string

const (
	CampusRabat     CampusCode = "rabat"
	CampusBenguerir CampusCode = "benguerir"
	CampusLaayoune  CampusCode = "laayoune"
)

func (c CampusCode) Valid() bool {
	switch c {
	case CampusRabat, CampusBenguerir, CampusLaayoune:
		return true
	}
	return false
}
