package policy

import "testing"

func TestEffectiveVaultRole(t *testing.T) {
	tests := []struct {
		vaultRole, orgRole, want string
	}{
		{"viewer", "developer", "viewer"},
		{"oncall", "viewer", "oncall"},
		{"developer", "admin", "admin"},
		{"", "admin", "admin"},
		{"", "owner", "owner"},
		{"admin", "owner", "owner"},
		{"owner", "developer", "owner"},
		{"", "developer", ""},
		{"", "viewer", ""},
		{"owner", "", ""},
		{"viewer", "", ""},
		{"bogus", "developer", ""},
	}
	for _, tt := range tests {
		if got := EffectiveVaultRole(tt.vaultRole, tt.orgRole); got != tt.want {
			t.Errorf("EffectiveVaultRole(%q, %q) = %q, want %q", tt.vaultRole, tt.orgRole, got, tt.want)
		}
	}
}

func TestRoleAllowsUnknownRoleDeniesEverything(t *testing.T) {
	for _, action := range []Action{ActionVaultRead, ActionSecretReveal, ActionVaultDelete, Action("anything")} {
		if RoleAllows("", action) || RoleAllows("superuser", action) {
			t.Errorf("unknown role must not be allowed %s", action)
		}
	}
	if RoleAllows(RoleOwner, Action("made:up")) {
		t.Error("unknown actions must be denied even for owners")
	}
}
