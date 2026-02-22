package crypto

import (
	"encoding/base64"
	"os"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	// Set up test encryption key before running tests (must be exactly 32 bytes)
	os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	code := m.Run()
	os.Exit(code)
}

func TestEncryptDecryptCredentials(t *testing.T) {
	// Sample credentials to test with
	credentials := map[string]string{
		"api_key":    "sk-1234567890abcdef",
		"secret_key": "secret123",
		"org_id":     "org-456789",
	}

	// Encrypt the credentials
	encrypted, err := EncryptCredentials(credentials)
	if err != nil {
		t.Fatalf("Failed to encrypt credentials: %v", err)
	}

	// Verify encrypted data is not empty and is valid base64
	if len(encrypted) == 0 {
		t.Fatal("Encrypted string should not be empty")
	}

	_, err = base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatal("Encrypted string should be valid base64")
	}

	// Decrypt the credentials
	decrypted, err := DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt credentials: %v", err)
	}

	// Verify decrypted data matches original
	if !reflect.DeepEqual(credentials, decrypted) {
		t.Errorf("Decrypted credentials don't match original.\nExpected: %v\nGot: %v", credentials, decrypted)
	}
}

func TestEncryptionKeyValidation(t *testing.T) {
	// Save the original key
	originalKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", originalKey)

	tests := []struct {
		name      string
		key       string
		wantError bool
	}{
		{
			name:      "missing key",
			key:       "",
			wantError: true,
		},
		{
			name:      "key too short",
			key:       "short-key",
			wantError: true,
		},
		{
			name:      "key too long",
			key:       "this-key-is-way-too-long-for-aes-256-encryption",
			wantError: true,
		},
		{
			name:      "key exactly 32 bytes",
			key:       "0123456789abcdef0123456789abcdef",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ENCRYPTION_KEY", tt.key)
			_, err := EncryptCredentials(map[string]string{"test": "value"})
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestEncryptInvalidData(t *testing.T) {
	tests := []struct {
		name      string
		encrypted string
		wantError bool
	}{
		{
			name:      "invalid base64",
			encrypted: "invalid-base64-string!@#",
			wantError: true,
		},
		{
			name:      "empty string",
			encrypted: "",
			wantError: true,
		},
		{
			name:      "too short ciphertext",
			encrypted: base64.StdEncoding.EncodeToString([]byte("short")),
			wantError: true,
		},
		{
			name:      "valid base64 but invalid ciphertext",
			encrypted: base64.StdEncoding.EncodeToString([]byte("this is not a valid encrypted payload")),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptCredentials(tt.encrypted)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}