package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// getKey returns the encryption key from environment or a default 32-byte key for AES-256
func getKey() []byte {
	if key := os.Getenv("ENCRYPTION_KEY"); key != "" {
		// If provided key is shorter than 32 bytes, pad with zeros
		// If longer, truncate to 32 bytes
		keyBytes := []byte(key)
		if len(keyBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded, keyBytes)
			return padded
		}
		return keyBytes[:32]
	}

	// Default 32-byte key for AES-256 (in production, this should be randomly generated and stored securely)
	return []byte("default-encryption-key-32-bytes!")
}

// EncryptCredentials encrypts a map of credentials using AES-256-GCM and returns base64 encoded string
func EncryptCredentials(creds map[string]string) (string, error) {
	// Convert credentials map to JSON
	jsonData, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Create AES cipher
	key := getKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nil, nonce, jsonData, nil)

	// Prepend nonce to ciphertext
	encrypted := append(nonce, ciphertext...)

	// Return base64 encoded result
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptCredentials decrypts a base64 encoded string back to credentials map using AES-256-GCM
func DecryptCredentials(encrypted string) (map[string]string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding: %w", err)
	}

	// Create AES cipher
	key := getKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check if data is long enough to contain nonce
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Unmarshal JSON back to map
	var creds map[string]string
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return creds, nil
}