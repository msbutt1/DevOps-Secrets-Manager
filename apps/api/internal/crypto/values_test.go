package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptValueRoundTrip(t *testing.T) {
	dek := testKey(t)
	for _, plaintext := range [][]byte{[]byte("postgres://user:pass@db/app"), {}, bytes.Repeat([]byte{0xff}, 64*1024)} {
		ciphertext, nonce, err := EncryptValue(plaintext, dek)
		if err != nil {
			t.Fatal(err)
		}
		if len(nonce) != 12 {
			t.Fatalf("nonce length %d", len(nonce))
		}
		if len(plaintext) > 0 && bytes.Contains(ciphertext, plaintext) {
			t.Fatal("ciphertext contains the plaintext")
		}
		got, err := DecryptValue(ciphertext, nonce, dek)
		if err != nil || !bytes.Equal(got, plaintext) {
			t.Fatalf("round trip failed: %v", err)
		}
	}
}

func TestEncryptValueUsesFreshNonces(t *testing.T) {
	dek := testKey(t)
	c1, n1, _ := EncryptValue([]byte("same value"), dek)
	c2, n2, _ := EncryptValue([]byte("same value"), dek)
	if bytes.Equal(n1, n2) || bytes.Equal(c1, c2) {
		t.Fatal("encrypting the same value twice produced the same nonce or ciphertext")
	}
}

func TestDecryptValueRejectsTampering(t *testing.T) {
	dek := testKey(t)
	ciphertext, nonce, err := EncryptValue([]byte("sk_live_do_not_leak"), dek)
	if err != nil {
		t.Fatal(err)
	}
	flip := func(b []byte, i int) []byte {
		out := bytes.Clone(b)
		out[i] ^= 0x01
		return out
	}
	otherCiphertext, otherNonce, _ := EncryptValue([]byte("another value"), dek)

	cases := map[string]struct{ ciphertext, nonce, key []byte }{
		"first ciphertext byte flipped": {flip(ciphertext, 0), nonce, dek},
		"authentication tag flipped":    {flip(ciphertext, len(ciphertext)-1), nonce, dek},
		"nonce flipped":                 {ciphertext, flip(nonce, 5), dek},
		"truncated ciphertext":          {ciphertext[:len(ciphertext)-4], nonce, dek},
		"empty ciphertext":              {nil, nonce, dek},
		"short nonce":                   {ciphertext, nonce[:8], dek},
		"another value's nonce":         {ciphertext, otherNonce, dek},
		"another value's ciphertext":    {otherCiphertext, nonce, dek},
		"wrong data key":                {ciphertext, nonce, testKey(t)},
		"invalid key length":            {ciphertext, nonce, dek[:20]},
	}
	for name, c := range cases {
		if got, err := DecryptValue(c.ciphertext, c.nonce, c.key); err == nil {
			t.Errorf("%s: decryption succeeded with %q", name, got)
		}
	}
}

func TestDecryptDEKRejectsTampering(t *testing.T) {
	kek, dek := testKey(t), testKey(t)
	wrapped, err := EncryptDEK(dek, kek)
	if err != nil {
		t.Fatal(err)
	}
	for i := range wrapped {
		tampered := bytes.Clone(wrapped)
		tampered[i] ^= 0x80
		if _, err := DecryptDEK(tampered, kek); err == nil {
			t.Fatalf("wrapped data key with byte %d flipped still decrypted", i)
		}
	}
}
