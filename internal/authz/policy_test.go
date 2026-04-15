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
	}
	for _, tt := range tests {
		if got := RoleAllows(tt.role, tt.action); got != tt.want {
			t.Fatalf("RoleAllows(%q, %q) = %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}
