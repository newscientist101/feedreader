// Package usenet provides shared types and helpers for the Usenet/NNTP
// integration. It is a leaf package with no dependency on package srv or
// srv/feeds so both can import it without import cycles.
package usenet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Crypto encrypts and decrypts NNTP credential secrets using AES-GCM.
// The application key must be exactly 32 bytes (AES-256).
type Crypto struct {
	key []byte
}

// NewCrypto creates a new Crypto from a 32-byte key.
// Returns an error if the key is not exactly 32 bytes.
func NewCrypto(key []byte) (*Crypto, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("usenet crypto: key must be 32 bytes, got %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &Crypto{key: k}, nil
}

// Encrypt encrypts plaintext and returns a hex-encoded string containing the
// random nonce prepended to the AES-GCM ciphertext.
// Returns an error if the plaintext is empty.
func (c *Crypto) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("usenet crypto: plaintext must not be empty")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("usenet crypto: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded nonce||ciphertext blob produced by Encrypt.
// Returns an error if the ciphertext is malformed or decryption fails.
func (c *Crypto) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", errors.New("usenet crypto: ciphertext must not be empty")
	}

	data, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: hex decode: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("usenet crypto: ciphertext too short")
	}

	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("usenet crypto: decrypt: %w", err)
	}

	return string(plaintext), nil
}
