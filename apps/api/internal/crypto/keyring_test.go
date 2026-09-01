package crypto

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestNewKeyringValidation(t *testing.T) {
	a, b := testKey(t), testKey(t)
	cases := map[string]func() error{
		"zero version":          func() error { _, err := NewKeyring(0, a, nil); return err },
		"short current key":     func() error { _, err := NewKeyring(1, a[:16], nil); return err },
		"previous repeats":      func() error { _, err := NewKeyring(2, a, map[int][]byte{2: b}); return err },
		"previous not positive": func() error { _, err := NewKeyring(2, a, map[int][]byte{0: b}); return err },
		"same key twice":        func() error { _, err := NewKeyring(2, a, map[int][]byte{1: a}); return err },
		"short previous key":    func() error { _, err := NewKeyring(2, a, map[int][]byte{1: b[:31]}); return err },
	}
	for name, build := range cases {
		if build() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := NewKeyring(2, a, map[int][]byte{1: b}); err != nil {
		t.Fatalf("valid keyring rejected: %v", err)
	}
}

func TestKeyringRotationRoundTrip(t *testing.T) {
	oldKey, newKey := testKey(t), testKey(t)
	dek := testKey(t)

	before := SingleKeyring(oldKey)
	wrapped, version, err := before.WrapDEK(dek)
	if err != nil || version != 1 {
		t.Fatalf("wrap: version %d, err %v", version, err)
	}

	during, err := NewKeyring(2, newKey, map[int][]byte{1: oldKey})
	if err != nil {
		t.Fatal(err)
	}
	unwrapped, err := during.UnwrapDEK(wrapped, version)
	if err != nil || !bytes.Equal(unwrapped, dek) {
		t.Fatalf("old data key not readable during rotation: %v", err)
	}
	rewrapped, newVersion, err := during.WrapDEK(unwrapped)
	if err != nil || newVersion != 2 {
		t.Fatalf("rewrap: version %d, err %v", newVersion, err)
	}

	after, _ := NewKeyring(2, newKey, nil)
	got, err := after.UnwrapDEK(rewrapped, newVersion)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("re-wrapped data key not readable with the new key alone: %v", err)
	}
	if _, err := after.UnwrapDEK(wrapped, 1); !errors.Is(err, ErrUnknownKEKVersion) {
		t.Fatalf("retired version should be unknown, got %v", err)
	}
	// A key recorded under the wrong version fails authentication rather than returning garbage.
	if _, err := during.UnwrapDEK(wrapped, 2); err == nil {
		t.Fatal("unwrapping with the wrong version's key succeeded")
	}
}

func TestParsePreviousKEKs(t *testing.T) {
	k1, k2 := hex.EncodeToString(testKey(t)), hex.EncodeToString(testKey(t))

	previous, err := ParsePreviousKEKs(" 1:" + k1 + " , 3:" + k2 + ",")
	if err != nil || len(previous) != 2 || previous[1] == nil || previous[3] == nil {
		t.Fatalf("parse: %v %v", previous, err)
	}
	if previous, err := ParsePreviousKEKs(""); err != nil || len(previous) != 0 {
		t.Fatalf("empty spec: %v %v", previous, err)
	}

	for _, bad := range []string{k1, "one:" + k1, "1:" + k1 + ",1:" + k2, "1:change-me", "1:" + strings.Repeat("0", 64)} {
		if _, err := ParsePreviousKEKs(bad); err == nil {
			t.Errorf("ParsePreviousKEKs(%q) should fail", bad)
		}
	}
}

func TestLoadKeyringFromEnv(t *testing.T) {
	current, old := hex.EncodeToString(testKey(t)), hex.EncodeToString(testKey(t))

	t.Setenv("MASTER_KEK", current)
	t.Setenv("MASTER_KEK_VERSION", "")
	t.Setenv("MASTER_KEK_PREVIOUS", "")
	k, err := LoadKeyringFromEnv()
	if err != nil || k.CurrentVersion() != 1 || len(k.Versions()) != 1 {
		t.Fatalf("defaults: %v %v", k, err)
	}

	t.Setenv("MASTER_KEK_VERSION", "2")
	t.Setenv("MASTER_KEK_PREVIOUS", "1:"+old)
	k, err = LoadKeyringFromEnv()
	if err != nil || k.CurrentVersion() != 2 || !k.Has(1) || !k.Has(2) {
		t.Fatalf("rotation config: %v %v", k, err)
	}

	t.Setenv("MASTER_KEK_VERSION", "two")
	if _, err := LoadKeyringFromEnv(); err == nil {
		t.Fatal("non-numeric version accepted")
	}
	t.Setenv("MASTER_KEK_VERSION", "2")
	t.Setenv("MASTER_KEK_PREVIOUS", "2:"+old)
	if _, err := LoadKeyringFromEnv(); err == nil {
		t.Fatal("previous key with the current version accepted")
	}
}
