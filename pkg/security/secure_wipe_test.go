package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureWipe_BasicFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "secret.txt")

	// Create a file with sensitive content
	sensitiveData := []byte("super secret API key: sk-ant-api03-xxxxxxxxxxxxx")
	if err := os.WriteFile(filePath, sensitiveData, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("test file not created")
	}

	// Secure wipe
	if err := SecureWipe(filePath); err != nil {
		t.Fatalf("SecureWipe failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist after wipe")
	}
}

func TestSecureWipe_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(filePath, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := SecureWipe(filePath); err != nil {
		t.Fatalf("SecureWipe failed on empty file: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("empty file should be removed after wipe")
	}
}

func TestSecureWipe_NonexistentFile(t *testing.T) {
	err := SecureWipe("/nonexistent/path/file.txt")
	if err != nil {
		t.Errorf("expected nil error for nonexistent file, got: %v", err)
	}
}

func TestSecureWipe_DirectoryReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	err := SecureWipe(tmpDir)
	if err == nil {
		t.Error("expected error when wiping a directory")
	}
}

func TestSecureWipeDir_BasicDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "secrets")
	os.MkdirAll(testDir, 0700)

	// Create files
	os.WriteFile(filepath.Join(testDir, "key1.enc"), []byte("secret1"), 0600)
	os.WriteFile(filepath.Join(testDir, "key2.enc"), []byte("secret2"), 0600)

	// Create subdirectory with files
	subDir := filepath.Join(testDir, "sub")
	os.MkdirAll(subDir, 0700)
	os.WriteFile(filepath.Join(subDir, "key3.enc"), []byte("secret3"), 0600)

	// Secure wipe
	if err := SecureWipeDir(testDir); err != nil {
		t.Fatalf("SecureWipeDir failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("directory should not exist after wipe")
	}
}

func TestSecureWipeDir_NonexistentDir(t *testing.T) {
	err := SecureWipeDir("/nonexistent/dir")
	if err != nil {
		t.Errorf("expected nil error for nonexistent dir, got: %v", err)
	}
}

func TestSecureWipeDir_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "single.txt")
	os.WriteFile(filePath, []byte("data"), 0600)

	// SecureWipeDir on a file should work (delegates to SecureWipe)
	if err := SecureWipeDir(filePath); err != nil {
		t.Fatalf("SecureWipeDir on file failed: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist after wipe")
	}
}

func TestSecureWipe_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.bin")

	// Create a 1MB file
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := SecureWipe(filePath); err != nil {
		t.Fatalf("SecureWipe failed on large file: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("large file should not exist after wipe")
	}
}
