package auth

import "testing"

func TestValidatePassword_MinLength(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("expected error for short password")
	}
	if err := ValidatePassword("longenough"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
