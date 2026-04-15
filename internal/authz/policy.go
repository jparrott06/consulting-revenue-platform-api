package authz

// Well-known actions for V1 RBAC checks.
const (
	ActionContextRead = "context.read"
	ActionAdminOps    = "admin.ops"
)

// RoleAllows returns whether a membership role may perform an action.
func RoleAllows(role, action string) bool {
	switch action {
	case ActionContextRead:
		return role == "owner" || role == "accountant" || role == "contractor"
	case ActionAdminOps:
		return role == "owner"
	default:
		return false
	}
}
