package auth

// WorkerRole is the fine-grained function a worker performs. It lives under
// the coarse `role = 'worker'` classification and is NULL for non-workers.
//
// This type is the single shared vocabulary for worker authorization: identity
// stores it on users, the JWT carries it, and downstream services (housing,
// transport, …) check against it instead of comparing raw strings.
//
// Keep these values in lockstep with the `worker_role` Postgres enum in
// services/identity/api/migrations (currently 0004_worker_roles).
type WorkerRole string

const (
	WorkerRoleHousingManager        WorkerRole = "housing_manager"
	WorkerRoleHousekeeperSupervisor WorkerRole = "housekeeper_supervisor"
	WorkerRoleHousekeeperOperator   WorkerRole = "housekeeper_operator"
)

// WorkerLevel is the seniority/authority axis within workers. It is *derived*
// from WorkerRole (a pure function, no stored column) so the two can never
// drift apart.
type WorkerLevel string

const (
	WorkerLevelManager    WorkerLevel = "manager"
	WorkerLevelSupervisor WorkerLevel = "supervisor"
	WorkerLevelOperator   WorkerLevel = "operator"
)

// Level returns the seniority level for a worker role. It returns "" for an
// empty or unknown role, which callers should treat as "not a worker".
func (r WorkerRole) Level() WorkerLevel {
	switch r {
	case WorkerRoleHousingManager:
		return WorkerLevelManager
	case WorkerRoleHousekeeperSupervisor:
		return WorkerLevelSupervisor
	case WorkerRoleHousekeeperOperator:
		return WorkerLevelOperator
	default:
		return ""
	}
}

// IsSupervisor reports whether the role is supervisor-level or above
// (manager or supervisor). Operators and non-workers return false.
func (r WorkerRole) IsSupervisor() bool {
	l := r.Level()
	return l == WorkerLevelManager || l == WorkerLevelSupervisor
}

// IsOperator reports whether the role is the least-privileged worker level.
func (r WorkerRole) IsOperator() bool {
	return r.Level() == WorkerLevelOperator
}

// Valid reports whether the role is one of the defined worker roles.
func (r WorkerRole) Valid() bool {
	switch r {
	case WorkerRoleHousingManager, WorkerRoleHousekeeperSupervisor, WorkerRoleHousekeeperOperator:
		return true
	default:
		return false
	}
}
