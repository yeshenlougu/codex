// Package store — API Key encryption
// Implements AES-256-GCM encryption for api_key storage (per SPEC §6.2).
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const encKeySize = 32 // AES-256

// KeyEncryption wraps AES-256-GCM encrypt/decrypt with a persistent key file.
type KeyEncryption struct {
	key []byte
}

// LoadOrCreateKey reads the encryption key from ~/.codex/.enckey,
// or generates a new one if it doesn't exist.
func LoadOrCreateKey(codexDir string) (*KeyEncryption, error) {
	keyPath := filepath.Join(codexDir, ".enckey")

	var key []byte
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == encKeySize {
		key = data
	} else {
		// Generate new key
		key = make([]byte, encKeySize)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate encryption key: %w", err)
		}
		if err := os.WriteFile(keyPath, key, 0600); err != nil {
			return nil, fmt.Errorf("write encryption key: %w", err)
		}
	}

	return &KeyEncryption{key: key}, nil
}

// Encrypt encrypts a plaintext string and returns a base64-encoded ciphertext.
// Format: base64(nonce + ciphertext). nonce is 12 bytes (GCM standard).
func (ke *KeyEncryption) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(ke.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext back to plaintext.
func (ke *KeyEncryption) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Not encrypted (legacy plaintext) — return as-is
		return encoded, nil
	}

	block, err := aes.NewCipher(ke.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		// Too short to be encrypted — return as-is (legacy plaintext)
		return encoded, nil
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Decryption failed — might be legacy plaintext stored as base64
		return encoded, nil
	}

	return string(plaintext), nil
}

// MaskKey returns a masked version of an API key for logging/display.
// Shows first 3 and last 4 characters (SPEC §6.8).
func MaskKey(key string) string {
	if len(key) <= 7 {
		return "***"
	}
	return key[:3] + "..." + key[len(key)-4:]
}
