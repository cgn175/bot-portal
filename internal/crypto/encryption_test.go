package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptCredentials(t *testing.T) {
	tests := []struct {
		name  string
		creds map[string]string
	}{
		{
			name: "simple API key",
			creds: map[string]string{
				"api_key": "sk-test12345",
			},
		},
		{
			name: "multiple credentials",
			creds: map[string]string{
				"api_key":     "sk-test12345",
				"api_secret":  "secret-value-789",
				"access_token": "token-abc-xyz",
			},
		},
		{
			name:  "empty credentials",
			creds: map[string]string{},
		},
		{
			name: "special characters",
			creds: map[string]string{
				"api_key": "sk-!@#$%^&*()_+-=[]{}|;':\",./<>?",
			},
		},
		{
			name: "unicode characters",
			creds: map[string]string{
				"api_key": "sk-测试-キー-🚀",
			},
		},
		{
			name: "long value",
			creds: map[string]string{
				"api_key": strings.Repeat("a", 1000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := EncryptCredentials(tt.creds)
			if err != nil {
				t.Fatalf("EncryptCredentials failed: %v", err)
			}

			// Verify encrypted is not empty and different from original
			if encrypted == "" {
				t.Error("encrypted string should not be empty")
			}

			// Decrypt
			decrypted, err := DecryptCredentials(encrypted)
			if err != nil {
				t.Fatalf("DecryptCredentials failed: %v", err)
			}

			// Verify decrypted matches original
			if len(decrypted) != len(tt.creds) {
				t.Errorf("decrypted length = %d, want %d", len(decrypted), len(tt.creds))
			}

			for key, expectedValue := range tt.creds {
				if decrypted[key] != expectedValue {
					t.Errorf("decrypted[%q] = %q, want %q", key, decrypted[key], expectedValue)
				}
			}
		})
	}
}

func TestEncryptDecryptCredentials_Deterministic(t *testing.T) {
	// Encryption should be non-deterministic (different ciphertext each time)
	creds := map[string]string{"api_key": "sk-test12345"}

	encrypted1, err := EncryptCredentials(creds)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}

	encrypted2, err := EncryptCredentials(creds)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}

	// Two encryptions of the same data should produce different ciphertexts
	// (due to random nonce)
	if encrypted1 == encrypted2 {
		t.Error("encryption should be non-deterministic (randomized nonce)")
	}

	// But both should decrypt to the same value
	decrypted1, err := DecryptCredentials(encrypted1)
	if err != nil {
		t.Fatalf("first decryption failed: %v", err)
	}

	decrypted2, err := DecryptCredentials(encrypted2)
	if err != nil {
		t.Fatalf("second decryption failed: %v", err)
	}

	if decrypted1["api_key"] != decrypted2["api_key"] {
		t.Error("both decryptions should produce the same value")
	}
}

func TestDecryptCredentials_InvalidData(t *testing.T) {
	tests := []struct {
		name      string
		encrypted string
		wantErr   bool
	}{
		{
			name:      "empty string",
			encrypted: "",
			wantErr:   false, // Empty returns empty map
		},
		{
			name:      "invalid base64",
			encrypted: "not-valid-base64!!!",
			wantErr:   true,
		},
		{
			name:      "too short data",
			encrypted: "dG9vLXNob3J0", // base64 of "too-short"
			wantErr:   true,
		},
		{
			name:      "corrupted ciphertext",
			encrypted: "dGVzdHRlc3R0ZXN0dGVzdHRlc3R0ZXN0dGVzdHRlc3Q=", // "test" repeated
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptCredentials(tt.encrypted)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEncryptDecryptString(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple string",
			plaintext: "hello world",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:      "unicode",
			plaintext: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptString(tt.plaintext)
			if err != nil {
				t.Fatalf("EncryptString failed: %v", err)
			}

			decrypted, err := DecryptString(encrypted)
			if err != nil {
				t.Fatalf("DecryptString failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("decrypted = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptCredentials_NilInput(t *testing.T) {
	// Should handle nil input gracefully
	encrypted, err := EncryptCredentials(nil)
	if err != nil {
		t.Fatalf("EncryptCredentials(nil) failed: %v", err)
	}

	decrypted, err := DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("DecryptCredentials failed: %v", err)
	}

	if decrypted == nil {
		t.Error("decrypted should not be nil, should be empty map")
	}

	if len(decrypted) != 0 {
		t.Errorf("decrypted should be empty, got %d items", len(decrypted))
	}
}

func BenchmarkEncryptCredentials(b *testing.B) {
	creds := map[string]string{
		"api_key":    "sk-test12345",
		"api_secret": "secret-value",
	}

	for i := 0; i < b.N; i++ {
		_, err := EncryptCredentials(creds)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptCredentials(b *testing.B) {
	creds := map[string]string{
		"api_key":    "sk-test12345",
		"api_secret": "secret-value",
	}

	encrypted, err := EncryptCredentials(creds)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		_, err := DecryptCredentials(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}
