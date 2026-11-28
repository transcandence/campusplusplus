package model

import "github.com/transcandence/campusplusplus/platform/go/auth"

type Account struct {
	UserID     int64
	CampusID   int64
	Role       string
	WorkerRole auth.WorkerRole
}

func (a Account) IsStudent() bool { return a.Role == "student" }

func (a Account) IsHousingManager() bool {
	return a.WorkerRole == auth.WorkerRoleHousingManager
}

func (a Account) IsHousekeeperSupervisor() bool {
	return a.WorkerRole == auth.WorkerRoleHousekeeperSupervisor
}

func (a Account) CanReadStays() bool {
	return a.IsHousingManager() || a.IsHousekeeperSupervisor()
}

func (a Account) CanManageStays() bool { return a.IsHousingManager() }
