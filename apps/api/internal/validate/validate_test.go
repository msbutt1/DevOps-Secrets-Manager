package validate

import (
	"strings"
	"testing"
)

func TestKeyName(t *testing.T) {
	for _, ok := range []string{"DATABASE_URL", "_private", "a1", strings.Repeat("K", 255)} {
		if err := KeyName(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1ABC", "WITH-DASH", "with space", "DOT.NAME", strings.Repeat("K", 256)} {
		if err := KeyName(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestLimits(t *testing.T) {
	long := strings.Repeat("x", MaxDescriptionLength+1)
	days := 0
	cases := []error{
		SecretValue(strings.Repeat("v", MaxSecretValueBytes+1)),
		Name("name", " "),
		Name("name", "bad\x00name"),
		Name("name", strings.Repeat("n", MaxNameLength+1)),
		Description(&long),
		Email("no-at-sign"),
		Email("a@"),
		RotationDays(&days),
		Metadata(map[string]interface{}{"team": 5}),
		Metadata(map[string]interface{}{"": "x"}),
	}
	for i, err := range cases {
		if err == nil || !IsValidationError(err) {
			t.Errorf("case %d should fail validation, got %v", i, err)
		}
	}
	days = 90
	if err := First(SecretValue("ok"), Name("name", "Payments API"), Email("a@b.c"), RotationDays(&days), Metadata(map[string]interface{}{"team": "payments"})); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
}

func TestPassword(t *testing.T) {
	accepted := []string{
		"Demo-Passw0rd!2026",
		"correct horse battery staple",
		"Integration-Test-Passw0rd!",
		"vK8#qLz2@wNp",
		strings.Repeat("ab1-Cd2_", 9), // 72 bytes
	}
	for _, pw := range accepted {
		if err := Password(pw, "salaar@demo.dev", "Salaar Butt"); err != nil {
			t.Errorf("Password(%q) = %v, want nil", pw, err)
		}
	}

	rejected := map[string]string{
		"Sh0rt-Pass!":                       "at least 12 characters",
		"password1234":                      "too common",
		"Password123456":                    "too common",
		"P@ssw0rd2024!!":                    "too common",
		"qwertyuiop123":                     "too common",
		"1234567890123":                     "too common",
		"iloveyou2026!":                     "too common",
		"aaaaaaaaaaaaaaaa":                  "5 different characters",
		"abababababab1212":                  "5 different characters",
		"Salaar-is-great-2026":              "name or email",
		"butt-secure-horse-42":              "name or email",
		strings.Repeat("ab1-Cd2_", 9) + "x": "at most 72 bytes",
	}
	for pw, want := range rejected {
		err := Password(pw, "salaar@demo.dev", "Salaar Butt")
		if err == nil || !strings.Contains(err.Error(), want) || !IsValidationError(err) {
			t.Errorf("Password(%q) = %v, want error containing %q", pw, err, want)
		}
	}
}

func TestCommonPasswordListLoaded(t *testing.T) {
	if len(commonPasswords) < 9000 {
		t.Fatalf("common password list has %d entries", len(commonPasswords))
	}
	if _, ok := commonPasswords["password"]; !ok {
		t.Fatal("list is missing \"password\"")
	}
}
