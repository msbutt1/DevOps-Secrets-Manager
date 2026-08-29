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
