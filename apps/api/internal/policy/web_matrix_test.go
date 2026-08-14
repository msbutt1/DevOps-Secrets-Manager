package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// TestMatrixMatchesWebRolePermissions parses ROLE_PERMISSIONS from the web app's types so the
// API and the UI can never disagree about what a role may do.
func TestMatrixMatchesWebRolePermissions(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "web", "src", "types", "api.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	block := regexp.MustCompile(`(?s)export const ROLE_PERMISSIONS[^=]*=\s*\{(.*?)\n\};`).FindSubmatch(source)
	if block == nil {
		t.Fatal("ROLE_PERMISSIONS not found in api.ts")
	}

	roleRe := regexp.MustCompile(`(?s)(\w+):\s*\{(.*?)\}`)
	fieldRe := regexp.MustCompile(`(\w+):\s*(true|false)`)
	seen := 0
	for _, role := range roleRe.FindAllSubmatch(block[1], -1) {
		name := string(role[1])
		web := map[string]bool{}
		for _, f := range fieldRe.FindAllSubmatch(role[2], -1) {
			web[string(f[1])] = string(f[2]) == "true"
		}
		api := PermissionsFor(name)
		if !IsValidRole(name) {
			t.Errorf("web role %q is unknown to the API", name)
			continue
		}
		seen++
		want := map[string]bool{
			"canRead": api.CanRead, "canWrite": api.CanWrite, "canReveal": api.CanReveal,
			"canManageMembers": api.CanManageMembers, "canDelete": api.CanDelete,
		}
		for key, value := range want {
			got, ok := web[key]
			if !ok || got != value {
				t.Errorf("%s.%s: web=%v (present=%v), api=%v", name, key, got, ok, value)
			}
		}
	}
	if seen != len(rolePermissions) {
		t.Errorf("web defines %d roles, API defines %d", seen, len(rolePermissions))
	}
}
