package srv

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// NNTPCrypto encrypts and decrypts NNTP credential secrets using AES-GCM.
// The application key must be exactly 32 bytes (AES-256).
type NNTPCrypto struct {
	key []byte
}

// NewNNTPCrypto creates a new NNTPCrypto from a 32-byte key.
// Returns an error if the key is not exactly 32 bytes.
func NewNNTPCrypto(key []byte) (*NNTPCrypto, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("nntp crypto: key must be 32 bytes, got %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &NNTPCrypto{key: k}, nil
}

// Encrypt encrypts plaintext and returns a hex-encoded string containing the
// random nonce prepended to the AES-GCM ciphertext.
// Returns an error if the plaintext is empty.
func (c *NNTPCrypto) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("nntp crypto: plaintext must not be empty")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nntp crypto: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded nonce||ciphertext blob produced by Encrypt.
// Returns an error if the ciphertext is malformed or decryption fails.
func (c *NNTPCrypto) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", errors.New("nntp crypto: ciphertext must not be empty")
	}

	data, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: hex decode: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("nntp crypto: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("nntp crypto: decrypt: %w", err)
	}

	return string(plaintext), nil
}
