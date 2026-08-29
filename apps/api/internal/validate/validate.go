// Package validate holds the input limits shared by the HTTP handlers.
package validate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits on user input. They are generous for real use and keep single requests bounded.
const (
	MaxNameLength        = 100
	MaxDescriptionLength = 1000
	MaxKeyNameLength     = 255
	MaxSecretValueBytes  = 64 * 1024
	MaxEmailLength       = 254
	MaxMetadataEntries   = 50
	MaxMetadataKeyLength = 100
	MaxMetadataValueLen  = 500
	MaxRotationDays      = 3650
)

// Error is a validation failure with a message safe to return to clients.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

func fail(format string, args ...any) error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// IsValidationError reports whether err came from this package.
func IsValidationError(err error) bool {
	var v *Error
	return errors.As(err, &v)
}

var keyNameRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// KeyName checks a secret key: an environment variable name of at most 255 characters.
func KeyName(key string) error {
	if key == "" || len(key) > MaxKeyNameLength || !keyNameRegexp.MatchString(key) {
		return fail("key_name must be a variable name of up to %d letters, digits and underscores, not starting with a digit", MaxKeyNameLength)
	}
	return nil
}

// SecretValue checks a secret value's size.
func SecretValue(value string) error {
	if len(value) > MaxSecretValueBytes {
		return fail("value must be at most %d bytes", MaxSecretValueBytes)
	}
	return nil
}

// Name checks a display name (vault, organization, person): 1-100 characters, no control characters.
func Name(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > MaxNameLength || hasControl(value) {
		return fail("%s must be 1-%d characters without control characters", field, MaxNameLength)
	}
	return nil
}

// Description checks an optional description.
func Description(value *string) error {
	if value != nil && utf8.RuneCountInString(*value) > MaxDescriptionLength {
		return fail("description must be at most %d characters", MaxDescriptionLength)
	}
	return nil
}

// Email performs a basic shape check; delivery proves the rest.
func Email(value string) error {
	value = strings.TrimSpace(value)
	at := strings.LastIndex(value, "@")
	if len(value) > MaxEmailLength || at < 1 || at == len(value)-1 || strings.ContainsAny(value, " \t\r\n") {
		return fail("email must be a valid address of at most %d characters", MaxEmailLength)
	}
	return nil
}

// MaxPasswordBytes is bcrypt's input limit; longer passwords would be silently truncated or rejected.
const MaxPasswordBytes = 72

// PasswordLength rejects passwords bcrypt cannot hash in full.
func PasswordLength(password string) error {
	if len(password) > MaxPasswordBytes {
		return fail("password must be at most %d bytes", MaxPasswordBytes)
	}
	return nil
}

// RotationDays checks an optional rotation interval.
func RotationDays(days *int) error {
	if days != nil && (*days < 1 || *days > MaxRotationDays) {
		return fail("rotation_interval_days must be between 1 and %d", MaxRotationDays)
	}
	return nil
}

// Metadata checks secret labels: at most 50 string entries with bounded keys and values.
func Metadata(metadata map[string]interface{}) error {
	if len(metadata) > MaxMetadataEntries {
		return fail("metadata can have at most %d entries", MaxMetadataEntries)
	}
	for k, v := range metadata {
		if k == "" || utf8.RuneCountInString(k) > MaxMetadataKeyLength {
			return fail("metadata keys must be 1-%d characters", MaxMetadataKeyLength)
		}
		s, ok := v.(string)
		if !ok {
			return fail("metadata value for %q must be a string", k)
		}
		if utf8.RuneCountInString(s) > MaxMetadataValueLen {
			return fail("metadata value for %q must be at most %d characters", k, MaxMetadataValueLen)
		}
	}
	return nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// First returns the first non-nil error.
func First(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
