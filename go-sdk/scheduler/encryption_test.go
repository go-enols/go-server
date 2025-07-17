package scheduler

import (
	"encoding/json"
	"testing"
)

// TestEncryptData tests the encryptData function
func TestEncryptData(t *testing.T) {
	testData := map[string]interface{}{
		"message": "hello world",
		"number":  42,
	}
	key := "test-key-123"

	// Test encryption
	encrypted, err := encryptData(testData, key)
	if err != nil {
		t.Fatalf("encryptData failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("encrypted data should not be empty")
	}

	t.Logf("Encrypted data: %s", encrypted)
}

// TestDecryptData tests the decryptData function
func TestDecryptData(t *testing.T) {
	testData := map[string]interface{}{
		"message": "hello world",
		"number":  42,
	}
	key := "test-key-123"

	// First encrypt the data
	encrypted, err := encryptData(testData, key)
	if err != nil {
		t.Fatalf("encryptData failed: %v", err)
	}

	// Then decrypt it
	decrypted, err := decryptData(encrypted, key)
	if err != nil {
		t.Fatalf("decryptData failed: %v", err)
	}

	// Verify the decrypted data matches original
	var result map[string]interface{}
	if err := json.Unmarshal(decrypted, &result); err != nil {
		t.Fatalf("unmarshal decrypted data failed: %v", err)
	}

	if result["message"] != testData["message"] {
		t.Fatalf("decrypted message mismatch: got %v, want %v", result["message"], testData["message"])
	}

	if result["number"].(float64) != float64(testData["number"].(int)) {
		t.Fatalf("decrypted number mismatch: got %v, want %v", result["number"], testData["number"])
	}

	t.Logf("Successfully decrypted data: %+v", result)
}

// TestSaltKey tests the saltKey function
func TestSaltKey(t *testing.T) {
	key := "test-key-123"
	salt := 12345

	// Test key salting
	saltedKey, err := saltKey(key, salt)
	if err != nil {
		t.Fatalf("saltKey failed: %v", err)
	}

	if saltedKey == "" {
		t.Fatal("salted key should not be empty")
	}

	if saltedKey == key {
		t.Fatal("salted key should be different from original key")
	}

	t.Logf("Salted key: %s", saltedKey)
}

// TestEncryptionConsistency tests that encryption produces consistent results
func TestEncryptionConsistency(t *testing.T) {
	testData := map[string]interface{}{
		"test": "data",
	}
	key := "consistent-key"

	// Encrypt the same data twice
	encrypted1, err := encryptData(testData, key)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}

	encrypted2, err := encryptData(testData, key)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}

	// Results should be identical due to deterministic IV
	if encrypted1 != encrypted2 {
		t.Fatal("encryption should be deterministic")
	}

	t.Logf("Consistent encryption result: %s", encrypted1)
}

// TestEncryptDecryptRoundTrip tests the complete encrypt-decrypt cycle
func TestEncryptDecryptRoundTrip(t *testing.T) {
	testCases := []map[string]interface{}{
		{"simple": "string"},
		{"number": 123, "float": 45.67},
		{"nested": map[string]interface{}{"inner": "value"}},
		{"array": []interface{}{1, 2, 3}},
	}

	key := "round-trip-key"

	for i, testData := range testCases {
		// Encrypt
		encrypted, err := encryptData(testData, key)
		if err != nil {
			t.Fatalf("case %d: encrypt failed: %v", i, err)
		}

		// Decrypt
		decrypted, err := decryptData(encrypted, key)
		if err != nil {
			t.Fatalf("case %d: decrypt failed: %v", i, err)
		}

		// Compare
		originalJSON, _ := json.Marshal(testData)
		if string(decrypted) != string(originalJSON) {
			t.Fatalf("case %d: round trip failed\noriginal: %s\ndecrypted: %s", i, originalJSON, decrypted)
		}

		t.Logf("Case %d: round trip successful", i)
	}
}
