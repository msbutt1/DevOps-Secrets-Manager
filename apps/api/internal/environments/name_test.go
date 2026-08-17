package environments

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	valid := []string{"dev", "staging", "production", "qa", "eu-west-1", "preview_42", "v1.2", " prod ", strings.Repeat("a", 64)}
	for _, name := range valid {
		if _, err := NormalizeName(name); err != nil {
			t.Errorf("%q should be valid: %v", name, err)
		}
	}
	invalid := []string{"", "  ", "Production", "-dev", ".hidden", "has space", "prod/eu", "ünïcode", strings.Repeat("a", 65)}
	for _, name := range invalid {
		if _, err := NormalizeName(name); err == nil {
			t.Errorf("%q should be rejected", name)
		}
	}
}

func TestNamePatternMatchesWebApp(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "web", "src", "lib", "environments.ts")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`ENVIRONMENT_NAME_PATTERN = /(.+)/;`).FindSubmatch(source)
	if m == nil {
		t.Fatal("ENVIRONMENT_NAME_PATTERN not found")
	}
	if string(m[1]) != NamePattern {
		t.Fatalf("web pattern %s differs from API pattern %s", m[1], NamePattern)
	}
}
