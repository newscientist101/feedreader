package srv

import (
	"strings"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	return []byte("12345678901234567890123456789012") // exactly 32 bytes
}

func TestNNTPCrypto_RoundTrip(t *testing.T) {
	c, err := NewNNTPCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewNNTPCrypto: %v", err)
	}

	plaintext := "supersecretpassword"
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == "" {
		t.Fatal("Encrypt returned empty string")
	}
	if strings.Contains(enc, plaintext) {
		t.Error("Encrypt output appears to contain plaintext")
	}

	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plaintext {
		t.Errorf("Decrypt = %q, want %q", dec, plaintext)
	}
}

func TestNNTPCrypto_UniqueNonce(t *testing.T) {
	c, err := NewNNTPCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewNNTPCrypto: %v", err)
	}

	enc1, err := c.Encrypt("password")
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	enc2, err := c.Encrypt("password")
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if enc1 == enc2 {
		t.Error("two encryptions of the same plaintext produced identical ciphertext (nonce reuse)")
	}
}

func TestNNTPCrypto_WrongKey(t *testing.T) {
	c1, err := NewNNTPCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewNNTPCrypto: %v", err)
	}

	enc, err := c1.Encrypt("password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrongKey := []byte("aaaabbbbccccddddaaaabbbbccccdddd") // different 32-byte key
	c2, err := NewNNTPCrypto(wrongKey)
	if err != nil {
		t.Fatalf("NewNNTPCrypto wrong key: %v", err)
	}

	_, err = c2.Decrypt(enc)
	if err == nil {
		t.Error("Decrypt with wrong key should fail but succeeded")
	}
}

func TestNNTPCrypto_EmptySecret(t *testing.T) {
	c, err := NewNNTPCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewNNTPCrypto: %v", err)
	}

	_, err = c.Encrypt("")
	if err == nil {
		t.Error("Encrypt empty string should fail but succeeded")
	}
}

func TestNNTPCrypto_MalformedCiphertext(t *testing.T) {
	c, err := NewNNTPCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewNNTPCrypto: %v", err)
	}

	// Non-hex input
	_, err = c.Decrypt("not-hex!!!")
	if err == nil {
		t.Error("Decrypt non-hex should fail but succeeded")
	}

	// Valid hex but too short to contain a nonce
	_, err = c.Decrypt("deadbeef")
	if err == nil {
		t.Error("Decrypt truncated ciphertext should fail but succeeded")
	}

	// Valid hex length but corrupted ciphertext (flip last byte)
	correct, _ := c.Encrypt("password")
	// flip last hex character
	flipped := correct[:len(correct)-1] + "0"
	if flipped[len(flipped)-1] == correct[len(correct)-1] {
		flipped = correct[:len(correct)-1] + "f"
	}
	_, err = c.Decrypt(flipped)
	if err == nil {
		t.Error("Decrypt corrupted ciphertext should fail but succeeded")
	}
}

func TestNNTPCrypto_KeyLength(t *testing.T) {
	_, err := NewNNTPCrypto([]byte("tooshort"))
	if err == nil {
		t.Error("NewNNTPCrypto with short key should fail but succeeded")
	}

	_, err = NewNNTPCrypto(make([]byte, 31))
	if err == nil {
		t.Error("NewNNTPCrypto with 31-byte key should fail but succeeded")
	}

	_, err = NewNNTPCrypto(make([]byte, 33))
	if err == nil {
		t.Error("NewNNTPCrypto with 33-byte key should fail but succeeded")
	}
}
