package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"testing"
)

// TestActionsMatchWebApp keeps the API's audit action names identical to AUDIT_ACTIONS in the
// web app, which filters and labels events by these names.
func TestActionsMatchWebApp(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "web", "src", "types", "api.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	block := regexp.MustCompile(`(?s)export const AUDIT_ACTIONS = \[(.*?)\] as const;`).FindSubmatch(source)
	if block == nil {
		t.Fatal("AUDIT_ACTIONS not found in api.ts")
	}
	var web []string
	for _, m := range regexp.MustCompile(`value:\s*'([^']+)'`).FindAllSubmatch(block[1], -1) {
		web = append(web, string(m[1]))
	}

	if !slices.Equal(web, Actions) {
		t.Fatalf("web AUDIT_ACTIONS %v\ndo not match API actions %v", web, Actions)
	}
}
