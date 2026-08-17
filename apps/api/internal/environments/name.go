package environments

import (
	"errors"
	"regexp"
	"strings"
)

// NamePattern is the rule for environment names: lowercase letters, digits, dots, hyphens and
// underscores, starting with a letter or digit, at most 64 characters. Any name that fits is
// allowed (development, staging, production, qa, eu-west-1, ...). The web app uses the same
// pattern in src/lib/environments.ts; a test keeps them identical.
const NamePattern = `^[a-z0-9][a-z0-9._-]{0,63}$`

var nameRegexp = regexp.MustCompile(NamePattern)

// ErrInvalidName is returned for names that do not match NamePattern.
var ErrInvalidName = errors.New("environment names must be 1-64 lowercase letters, digits, dots, hyphens or underscores, starting with a letter or digit")

// NormalizeName trims surrounding whitespace and validates the name.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !nameRegexp.MatchString(name) {
		return "", ErrInvalidName
	}
	return name, nil
}
