package authz

import "testing"

func TestRoleAllows_Matrix(t *testing.T) {
	tests := []struct {
		role   string
		action string
		want   bool
	}{
		{"owner", ActionContextRead, true},
		{"owner", ActionAdminOps, true},
		{"accountant", ActionContextRead, true},
		{"accountant", ActionAdminOps, false},
		{"contractor", ActionContextRead, true},
		{"contractor", ActionAdminOps, false},
		{"guest", ActionContextRead, false},
		{"guest", ActionAdminOps, false},
		{"owner", "unknown.action", false},
		{"owner", ActionMembershipRead, true},
		{"owner", ActionMembershipWrite, true},
		{"accountant", ActionMembershipRead, true},
		{"accountant", ActionMembershipWrite, false},
		{"contractor", ActionMembershipRead, false},
		{"contractor", ActionMembershipWrite, false},
		{"owner", ActionClientRead, true},
		{"accountant", ActionClientRead, true},
		{"contractor", ActionClientRead, true},
		{"owner", ActionClientWrite, true},
		{"accountant", ActionClientWrite, true},
		{"contractor", ActionClientWrite, false},
		{"owner", ActionProjectRead, true},
		{"accountant", ActionProjectRead, true},
		{"contractor", ActionProjectRead, true},
		{"owner", ActionProjectWrite, true},
		{"accountant", ActionProjectWrite, true},
		{"contractor", ActionProjectWrite, false},
	}
	for _, tt := range tests {
		if got := RoleAllows(tt.role, tt.action); got != tt.want {
			t.Fatalf("RoleAllows(%q, %q) = %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}
