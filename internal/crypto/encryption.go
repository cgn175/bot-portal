package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// getKey returns the encryption key from environment
func getKey() ([]byte, error) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable not set")
	}

	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes for AES-256, got %d bytes", len(keyBytes))
	}

	return keyBytes, nil
}

// encrypt encrypts plaintext using AES-256-GCM.
// Returns a base64-encoded string containing the nonce and ciphertext.
func encrypt(plaintext []byte) (string, error) {
	key, err := getKey()
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a base64-encoded ciphertext using AES-256-GCM.
// Returns the original plaintext bytes.
func decrypt(encrypted string) ([]byte, error) {
	if encrypted == "" {
		return nil, nil
	}
	
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	
	key, err := getKey()
	if err != nil {
		return nil, err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptCredentials encrypts a map of credentials
func EncryptCredentials(creds map[string]string) (string, error) {
	if creds == nil {
		creds = make(map[string]string)
	}
	data, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return encrypt(data)
}

// DecryptCredentials decrypts a base64-encoded ciphertext back to a credentials map
func DecryptCredentials(encrypted string) (map[string]string, error) {
	creds := make(map[string]string)
	if encrypted == "" {
		return creds, nil
	}
	
	plaintext, err := decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return creds, nil
}

// EncryptString encrypts a plain string using AES-256-GCM
// Returns a base64-encoded string.
func EncryptString(plaintext string) (string, error) {
	return encrypt([]byte(plaintext))
}

// DecryptString decrypts a base64-encoded ciphertext to a plain string.
func DecryptString(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	plaintext, err := decrypt(encrypted)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
