package authz

// Well-known actions for V1 RBAC checks.
const (
	ActionContextRead     = "context.read"
	ActionAdminOps        = "admin.ops"
	ActionMembershipRead  = "membership.read"
	ActionMembershipWrite = "membership.write"
	ActionClientRead      = "client.read"
	ActionClientWrite     = "client.write"
)

// RoleAllows returns whether a membership role may perform an action.
func RoleAllows(role, action string) bool {
	switch action {
	case ActionContextRead:
		return role == "owner" || role == "accountant" || role == "contractor"
	case ActionAdminOps:
		return role == "owner"
	case ActionMembershipRead:
		return role == "owner" || role == "accountant"
	case ActionMembershipWrite:
		return role == "owner"
	case ActionClientRead:
		return role == "owner" || role == "accountant" || role == "contractor"
	case ActionClientWrite:
		return role == "owner" || role == "accountant"
	default:
		return false
	}
}
