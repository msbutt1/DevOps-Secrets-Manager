package crypto

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ErrUnknownKEKVersion means data was wrapped with a master key version that is not configured.
var ErrUnknownKEKVersion = errors.New("master key version is not configured")

// Keyring holds the current master key (KEK) and, during a rotation, earlier ones. New vault
// data keys are always wrapped with the current key; stored data keys record the version that
// wrapped them, so they can still be unwrapped until they are re-wrapped with the current key.
type Keyring struct {
	current int
	keys    map[int][]byte
}

// NewKeyring builds a keyring. Versions are positive, and no two versions may share a key.
func NewKeyring(currentVersion int, current []byte, previous map[int][]byte) (*Keyring, error) {
	if currentVersion < 1 {
		return nil, fmt.Errorf("MASTER_KEK_VERSION must be a positive integer, got %d", currentVersion)
	}
	if len(current) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(current))
	}
	keys := map[int][]byte{currentVersion: current}
	for version, key := range previous {
		if version < 1 {
			return nil, fmt.Errorf("MASTER_KEK_PREVIOUS versions must be positive integers, got %d", version)
		}
		if version == currentVersion {
			return nil, fmt.Errorf("MASTER_KEK_PREVIOUS repeats the current version %d", version)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("master key version %d must be 32 bytes, got %d", version, len(key))
		}
		for other, existing := range keys {
			if subtle.ConstantTimeCompare(existing, key) == 1 {
				return nil, fmt.Errorf("master key versions %d and %d are the same key", other, version)
			}
		}
		keys[version] = key
	}
	return &Keyring{current: currentVersion, keys: keys}, nil
}

// SingleKeyring is a keyring with only one key at version 1, for tests and tools.
func SingleKeyring(kek []byte) *Keyring {
	k, err := NewKeyring(1, kek, nil)
	if err != nil {
		panic(err)
	}
	return k
}

// CurrentVersion is the version new data keys are wrapped with.
func (k *Keyring) CurrentVersion() int { return k.current }

// Versions lists the configured versions in ascending order.
func (k *Keyring) Versions() []int {
	versions := make([]int, 0, len(k.keys))
	for v := range k.keys {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions
}

// Has reports whether the version is configured.
func (k *Keyring) Has(version int) bool {
	_, ok := k.keys[version]
	return ok
}

// WrapDEK encrypts a vault data key with the current master key.
func (k *Keyring) WrapDEK(dek []byte) (wrapped []byte, version int, err error) {
	wrapped, err = EncryptDEK(dek, k.keys[k.current])
	return wrapped, k.current, err
}

// UnwrapDEK decrypts a vault data key wrapped with the given master key version.
func (k *Keyring) UnwrapDEK(wrapped []byte, version int) ([]byte, error) {
	key, ok := k.keys[version]
	if !ok {
		return nil, fmt.Errorf("%w: version %d (add it to MASTER_KEK_PREVIOUS)", ErrUnknownKEKVersion, version)
	}
	return DecryptDEK(wrapped, key)
}

// LoadKeyringFromEnv reads MASTER_KEK, MASTER_KEK_VERSION (default 1) and MASTER_KEK_PREVIOUS,
// a comma-separated list of version:hexkey pairs still needed to read data during a rotation.
func LoadKeyringFromEnv() (*Keyring, error) {
	current, err := ParseKEK(os.Getenv("MASTER_KEK"))
	if err != nil {
		return nil, err
	}

	version := 1
	if raw := strings.TrimSpace(os.Getenv("MASTER_KEK_VERSION")); raw != "" {
		if version, err = strconv.Atoi(raw); err != nil {
			return nil, fmt.Errorf("MASTER_KEK_VERSION must be a positive integer")
		}
	}

	previous, err := ParsePreviousKEKs(os.Getenv("MASTER_KEK_PREVIOUS"))
	if err != nil {
		return nil, err
	}
	return NewKeyring(version, current, previous)
}

// ParsePreviousKEKs parses "1:hexkey,2:hexkey". Keys are checked like MASTER_KEK.
func ParsePreviousKEKs(spec string) (map[int][]byte, error) {
	previous := map[int][]byte{}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		rawVersion, rawKey, ok := strings.Cut(entry, ":")
		version, err := strconv.Atoi(strings.TrimSpace(rawVersion))
		if !ok || err != nil {
			return nil, fmt.Errorf("MASTER_KEK_PREVIOUS entries must look like version:hexkey")
		}
		if _, dup := previous[version]; dup {
			return nil, fmt.Errorf("MASTER_KEK_PREVIOUS lists version %d twice", version)
		}
		key, err := ParseKEK(strings.TrimSpace(rawKey))
		if err != nil {
			return nil, fmt.Errorf("MASTER_KEK_PREVIOUS version %d: %s", version, strings.TrimPrefix(err.Error(), "MASTER_KEK "))
		}
		previous[version] = key
	}
	return previous, nil
}
