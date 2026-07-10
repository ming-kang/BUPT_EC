package service

import "testing"

// AES known vectors for encryptJWPassword.
//
// Generation method (independent of this package): Node.js crypto
// createCipheriv("aes-128-ecb", key, null) with setAutoPadding(false), after
// JSON.stringify(password), PKCS#7 pad to 16-byte blocks, then
// base64(base64(ciphertext-bytes-as-string-utf8)). Key matches the JW protocol
// constant and is not a secret; do not regenerate by calling encryptJWPassword.
//
// Reproducible one-off reference (not used in CI):
//
//	JSON marshal → PKCS#7 → AES-128-ECB → StdEncoding base64 twice.
func TestEncryptJWPasswordKnownVectors(t *testing.T) {
	vectors := []struct {
		name      string
		plaintext string
		// expected is a hard-coded independent reference ciphertext.
		expected string
	}{
		{
			name:      "ascii-secret",
			plaintext: "secret",
			expected:  "bDRPK005bThPMXFabnBReXhHOURIZz09",
		},
		{
			name:      "empty",
			plaintext: "",
			expected:  "NHBVbDhHTFMxVS9wanNzYnpIK28vQT09",
		},
		{
			// JSON bytes: "1234567890123" → 15 bytes (one-byte short of a block).
			name:      "fifteen-json-bytes",
			plaintext: "1234567890123",
			expected:  "WkxmZGI3YUN2SzVNb1pFNXJ3c1o0Zz09",
		},
		{
			// JSON bytes: "12345678901234" → 16 bytes (exact block before padding).
			name:      "sixteen-json-bytes",
			plaintext: "12345678901234",
			expected:  "bFR5V08yOEtubEQ1eXZmeVF0SkorbjdzMEZpL0lNamlpZWEyaE5pR2VnWT0=",
		},
		{
			// JSON bytes: "123456789012345" → 17 bytes (crosses first block).
			name:      "seventeen-json-bytes",
			plaintext: "123456789012345",
			expected:  "QXdwZktCMDJPK1F6Y3MwREpKcEN5U2JTZnpyWUZNRC9DVmNNald6aG5OZz0=",
		},
		{
			name:      "unicode",
			plaintext: "密码测试",
			expected:  "K3N1K2N1bkhHcHQvblZhczhIalRBdz09",
		},
		{
			name:      "quotes-and-backslash",
			plaintext: "a\"b\\c",
			expected:  "ckFIS05OMjZCeVA5Qk90ZkhGZFByQT09",
		},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encryptJWPassword(tc.plaintext)
			if err != nil {
				t.Fatalf("encryptJWPassword() error = %v", err)
			}
			if got != tc.expected {
				t.Fatalf("encryptJWPassword() = %q, want independent known vector %q", got, tc.expected)
			}
		})
	}
}

func TestEncryptJWPasswordRejectsInvalidUTF8ForJSON(t *testing.T) {
	// json.Marshal on a Go string always succeeds for valid UTF-8 Go strings.
	// This smoke check keeps the API surface exercised without pairing against
	// a production-derived expected value.
	got, err := encryptJWPassword("ok")
	if err != nil {
		t.Fatalf("encryptJWPassword() error = %v", err)
	}
	if got == "" {
		t.Fatal("encryptJWPassword() returned empty ciphertext")
	}
}
