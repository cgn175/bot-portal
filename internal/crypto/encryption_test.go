package crypto

import (
	"encoding/base64"
	"reflect"
	"testing"
)

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