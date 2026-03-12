# Code Review: internal/crypto/encryption.go

## Summary
The changes introduce several **Critical Security Vulnerabilities**. While the intent was likely to make local development easier by providing a default key and padding it to the required 32 bytes, these changes compromise the security of the encryption module and violate our technical standards. 

**Status:** ❌ Rejected

## Critical Issues

### 1. Hardcoded Default Key
```go
key = "bot-portal-default-dev-key-32b"
```
Hardcoding encryption keys in the codebase is a violation of our security guardrails and code review checklist (`[ ] No secrets, API keys, or credentials in code`). Even if intended for development, this key will inevitably end up used in environments it shouldn't be, leading to insecure ciphertexts.
* **Fix**: Revert this. `getKey()` must return an error if `ENCRYPTION_KEY` is not set in the environment.

### 2. Unsafe Key Padding and Truncation
```go
if len(keyBytes) < 32 {
    padded := make([]byte, 32)
    copy(padded, keyBytes)
    return padded
}
if len(keyBytes) > 32 {
    return keyBytes[:32]
}
```
Cryptographic keys **must never** be artificially padded or truncated to fit block size requirements. If a user provides a 10-byte key, it has 80 bits of entropy at most. Padding it with 22 bytes of zeros does not magically make it AES-256 compliant; it makes it an easily brute-forceable AES-256 key. Truncating a key throws away entropy.
* **Fix**: Revert to the strict length check. The system should fast-fail if the key is not exactly 32 bytes.

### 3. Masking `getKey()` Errors
```go
func getKey() []byte { ... }
```
The signature of `getKey()` was changed to drop the `error` return type. This means downstream functions (`EncryptCredentials`, `DecryptCredentials`) blindly proceed assuming they have a valid cryptographically secure key, which obscures potential initialization failures.
* **Fix**: Restore `func getKey() ([]byte, error)` and explicitly handle the error when creating the cipher lock.

## Minor Notes
- The addition of `EncryptString` and `DecryptString` wrappers is fine, provided the underlying credential functions are secure. However, using a wrapper object `{"value": plaintext}` adds unnecessary JSON overhead just to encrypt a string. It would be better to extract the core AES-GCM logic into private `encrypt([]byte)` / `decrypt([]byte)` functions that both maps and strings can use directly.

## Action Items
Please update the PR/commit by:
1. Removing the default dev key.
2. Removing key padding and truncation logic.
3. Restoring the error return parameter from `getKey()`.
4. (Optional) Refactoring the shared AES-GCM logic for efficiency.
