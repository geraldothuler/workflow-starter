package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedFileProvider_Name(t *testing.T) {
	p := NewEncryptedFileProvider(t.TempDir(), "testkey")
	if p.Name() != "encrypted-file" {
		t.Errorf("expected 'encrypted-file', got %q", p.Name())
	}
}

func TestEncryptedFileProvider_Available_WithKey(t *testing.T) {
	p := NewEncryptedFileProvider(t.TempDir(), "testkey")
	if !p.Available() {
		t.Error("expected available with master key set")
	}
}

func TestEncryptedFileProvider_Available_WithoutKey(t *testing.T) {
	// Clear env var so NewEncryptedFileProvider doesn't pick it up
	old := os.Getenv("WTB_MASTER_KEY")
	os.Unsetenv("WTB_MASTER_KEY")
	defer func() {
		if old != "" {
			os.Setenv("WTB_MASTER_KEY", old)
		}
	}()

	p := NewEncryptedFileProvider(t.TempDir(), "")
	if p.Available() {
		t.Error("expected not available without master key")
	}
}

func TestEncryptedFileProvider_StoreAndResolve(t *testing.T) {
	dir := t.TempDir()
	masterKey := "test-master-key-12345"
	p := NewEncryptedFileProvider(dir, masterKey)
	ctx := context.Background()

	// Store a credential
	err := p.Store(ctx, "MY_SECRET", "super-secret-value")
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// Verify file was created in secrets dir
	secretsDir := dir
	files, err := os.ReadDir(secretsDir)
	if err != nil {
		t.Fatalf("failed to read secrets dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file in secrets dir, got %d", len(files))
	}

	// Verify file has .enc extension
	if filepath.Ext(files[0].Name()) != ".enc" {
		t.Errorf("expected .enc extension, got %q", files[0].Name())
	}

	// Verify file permissions are restrictive (0600)
	info, err := os.Stat(filepath.Join(secretsDir, files[0].Name()))
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}

	// Resolve should return the original value
	cred, err := p.Resolve(ctx, "MY_SECRET")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if cred.Value != "super-secret-value" {
		t.Errorf("expected 'super-secret-value', got %q", cred.Value)
	}
	if cred.Source != "encrypted-file" {
		t.Errorf("expected source 'encrypted-file', got %q", cred.Source)
	}
}

func TestEncryptedFileProvider_Resolve_NotFound(t *testing.T) {
	p := NewEncryptedFileProvider(t.TempDir(), "testkey")
	ctx := context.Background()

	_, err := p.Resolve(ctx, "NONEXISTENT")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEncryptedFileProvider_Resolve_WrongKey(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Store with one key
	p1 := NewEncryptedFileProvider(dir, "correct-key")
	if err := p1.Store(ctx, "TOKEN", "value"); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// Try to resolve with different key
	p2 := NewEncryptedFileProvider(dir, "wrong-key")
	_, err := p2.Resolve(ctx, "TOKEN")
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestEncryptedFileProvider_Store_NoMasterKey(t *testing.T) {
	p := &EncryptedFileProvider{
		secretsDir: t.TempDir(),
		masterKey:  "",
	}
	ctx := context.Background()

	err := p.Store(ctx, "TOKEN", "value")
	if err == nil {
		t.Fatal("expected error without master key")
	}
}

func TestEncryptedFileProvider_Resolve_NoMasterKey(t *testing.T) {
	p := &EncryptedFileProvider{
		secretsDir: t.TempDir(),
		masterKey:  "",
	}
	ctx := context.Background()

	_, err := p.Resolve(ctx, "TOKEN")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound without master key, got %v", err)
	}
}

func TestEncryptedFileProvider_OverwriteCredential(t *testing.T) {
	dir := t.TempDir()
	p := NewEncryptedFileProvider(dir, "testkey")
	ctx := context.Background()

	// Store initial value
	if err := p.Store(ctx, "TOKEN", "original"); err != nil {
		t.Fatalf("first store failed: %v", err)
	}

	// Overwrite
	if err := p.Store(ctx, "TOKEN", "updated"); err != nil {
		t.Fatalf("second store failed: %v", err)
	}

	// Should return updated value
	cred, err := p.Resolve(ctx, "TOKEN")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if cred.Value != "updated" {
		t.Errorf("expected 'updated', got %q", cred.Value)
	}
}
