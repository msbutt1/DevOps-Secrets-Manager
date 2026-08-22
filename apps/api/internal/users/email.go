package users

import "strings"

// NormalizeEmail trims and lower-cases an address. Accounts are looked up and stored by the
// normalised form, so the same person cannot register twice with different capitalisation.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
