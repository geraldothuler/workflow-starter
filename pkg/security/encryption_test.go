package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEncryptionManager(t *testing.T) {
	em := NewEncryptionManager(true)
	if em == nil {
		t.Fatal("expected non-nil manager")
	}
	if !em.enabled {
		t.Error("expected enabled=true")
	}
}

func TestNewEncryptionManager_Disabled(t *testing.T) {
	em := NewEncryptionManager(false)
	if em.enabled {
		t.Error("expected enabled=false")
	}
}

func TestEncrypt_Decrypt_Roundtrip(t *testing.T) {
	em := NewEncryptionManager(true)
	original := []byte("Hello, World! This is secret data.")
	password := "strongpassword1234"

	encrypted, err := em.Encrypt(original, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("encrypted data should not be empty")
	}
	if string(encrypted) == string(original) {
		t.Error("encrypted data should differ from original")
	}

	decrypted, err := em.Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if string(decrypted) != string(original) {
		t.Errorf("expected %q, got %q", string(original), string(decrypted))
	}
}

func TestEncrypt_DifferentEachTime(t *testing.T) {
	em := NewEncryptionManager(true)
	data := []byte("same data")
	password := "samepassword1234"

	enc1, _ := em.Encrypt(data, password)
	enc2, _ := em.Encrypt(data, password)

	if string(enc1) == string(enc2) {
		t.Error("encrypting same data twice should produce different ciphertext (random salt/nonce)")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	em := NewEncryptionManager(true)
	_, err := em.Decrypt([]byte("not-valid-base64!!!"), "password")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	em := NewEncryptionManager(true)
	// Valid base64 but too short once decoded (less than 32 bytes for salt)
	_, err := em.Decrypt([]byte("YWJj"), "password") // "abc" in base64
	if err == nil {
		t.Error("expected error for too short data")
	}
}

func TestDecrypt_WrongPassword(t *testing.T) {
	em := NewEncryptionManager(true)
	data := []byte("secret message")

	encrypted, err := em.Encrypt(data, "correctpassword!")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = em.Decrypt(encrypted, "wrongpassword!!!")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestEncryptFile_DecryptFile_Roundtrip(t *testing.T) {
	em := NewEncryptionManager(true)
	tmpDir := t.TempDir()

	original := "This is a file with sensitive content."
	inputPath := filepath.Join(tmpDir, "input.txt")
	encryptedPath := filepath.Join(tmpDir, "encrypted.bin")
	decryptedPath := filepath.Join(tmpDir, "decrypted.txt")
	password := "filepassword1234"

	os.WriteFile(inputPath, []byte(original), 0644)

	err := em.EncryptFile(inputPath, encryptedPath, password)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Verify encrypted file exists
	if _, err := os.Stat(encryptedPath); os.IsNotExist(err) {
		t.Fatal("encrypted file should exist")
	}

	err = em.DecryptFile(encryptedPath, decryptedPath, password)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	decrypted, _ := os.ReadFile(decryptedPath)
	if string(decrypted) != original {
		t.Errorf("expected %q, got %q", original, string(decrypted))
	}
}

func TestEncryptFile_Disabled(t *testing.T) {
	em := NewEncryptionManager(false)
	err := em.EncryptFile("input", "output", "pass")
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestDecryptFile_Disabled(t *testing.T) {
	em := NewEncryptionManager(false)
	err := em.DecryptFile("input", "output", "pass")
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestEncryptFile_NonExistent(t *testing.T) {
	em := NewEncryptionManager(true)
	err := em.EncryptFile("/nonexistent/file.txt", "/tmp/out.bin", "pass")
	if err == nil {
		t.Error("expected error for nonexistent input file")
	}
}

func TestEncryptJSON_DecryptJSON_Roundtrip(t *testing.T) {
	em := NewEncryptionManager(true)
	password := "jsonpassword1234"

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := TestData{Name: "test", Value: 42}

	encrypted, err := em.EncryptJSON(original, password)
	if err != nil {
		t.Fatalf("EncryptJSON failed: %v", err)
	}

	var decoded TestData
	err = em.DecryptJSON(encrypted, password, &decoded)
	if err != nil {
		t.Fatalf("DecryptJSON failed: %v", err)
	}

	if decoded.Name != original.Name || decoded.Value != original.Value {
		t.Errorf("expected %+v, got %+v", original, decoded)
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(key))
	}

	// Keys should be different each time
	key2, _ := GenerateKey()
	if string(key) == string(key2) {
		t.Error("keys should be random and different")
	}
}

func TestKeyStrength(t *testing.T) {
	tests := []struct {
		password string
		ok       bool
		contains string
	}{
		{"short", false, "Muito fraca"},
		{"eightchr", false, "Muito fraca"},
		{"twelvechar!", false, "Fraca"},
		{"fifteenchar!!!", true, "Média"},
		{"sixteencharacte!", true, "Forte"},
		{"this-is-a-very-strong-password-indeed", true, "Forte"},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			strength, ok := KeyStrength(tt.password)
			if ok != tt.ok {
				t.Errorf("password %q: expected ok=%v, got %v (strength: %s)", tt.password, tt.ok, ok, strength)
			}
		})
	}
}
