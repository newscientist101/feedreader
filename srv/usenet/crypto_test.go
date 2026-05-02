package usenet_test

import (
	"strings"
	"testing"

	"github.com/newscientist101/feedreader/srv/usenet"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	return []byte("12345678901234567890123456789012") // exactly 32 bytes
}

func TestCrypto_RoundTrip(t *testing.T) {
	c, err := usenet.NewCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
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

func TestCrypto_UniqueNonce(t *testing.T) {
	c, err := usenet.NewCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
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

func TestCrypto_WrongKey(t *testing.T) {
	c1, err := usenet.NewCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
	}

	enc, err := c1.Encrypt("password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrongKey := []byte("aaaabbbbccccddddaaaabbbbccccdddd") // different 32-byte key
	c2, err := usenet.NewCrypto(wrongKey)
	if err != nil {
		t.Fatalf("NewCrypto wrong key: %v", err)
	}

	_, err = c2.Decrypt(enc)
	if err == nil {
		t.Error("Decrypt with wrong key should fail but succeeded")
	}
}

func TestCrypto_EmptySecret(t *testing.T) {
	c, err := usenet.NewCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
	}

	_, err = c.Encrypt("")
	if err == nil {
		t.Error("Encrypt empty string should fail but succeeded")
	}
}

func TestCrypto_MalformedCiphertext(t *testing.T) {
	c, err := usenet.NewCrypto(testKey(t))
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
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

func TestCrypto_KeyLength(t *testing.T) {
	_, err := usenet.NewCrypto([]byte("tooshort"))
	if err == nil {
		t.Error("NewCrypto with short key should fail but succeeded")
	}

	_, err = usenet.NewCrypto(make([]byte, 31))
	if err == nil {
		t.Error("NewCrypto with 31-byte key should fail but succeeded")
	}

	_, err = usenet.NewCrypto(make([]byte, 33))
	if err == nil {
		t.Error("NewCrypto with 33-byte key should fail but succeeded")
	}
}
