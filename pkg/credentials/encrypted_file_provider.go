package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptedFileProvider stores credentials as AES-256-GCM encrypted files.
// Each credential is stored in .workflow/secrets/<sha256(name)>.enc.
// Uses PBKDF2 key derivation + AES-256-GCM (self-contained, no external deps).
type EncryptedFileProvider struct {
	secretsDir string // absolute path to secrets directory
	masterKey  string // master password for encryption/decryption
}

// NewEncryptedFileProvider creates an encrypted file credential provider.
// secretsDir is the directory where encrypted credential files are stored.
// masterKey is the password used for AES-256-GCM encryption.
// If masterKey is empty, it tries WTB_MASTER_KEY env var.
func NewEncryptedFileProvider(secretsDir, masterKey string) *EncryptedFileProvider {
	if masterKey == "" {
		masterKey = os.Getenv("WTB_MASTER_KEY")
	}

	return &EncryptedFileProvider{
		secretsDir: secretsDir,
		masterKey:  masterKey,
	}
}

// Name returns "encrypted-file".
func (p *EncryptedFileProvider) Name() string { return "encrypted-file" }

// Resolve reads and decrypts a credential from its file.
// Returns ErrNotFound if the file does not exist.
func (p *EncryptedFileProvider) Resolve(_ context.Context, name string) (*Credential, error) {
	if p.masterKey == "" {
		return nil, ErrNotFound // cannot decrypt without master key
	}

	filePath := p.credentialPath(name)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read credential file: %w", err)
	}

	decrypted, err := decrypt(data, p.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credential %q: %w", name, err)
	}

	return &Credential{
		Name:   name,
		Value:  string(decrypted),
		Source: "encrypted-file",
	}, nil
}

// Store encrypts and saves a credential to a file with restrictive permissions.
func (p *EncryptedFileProvider) Store(_ context.Context, name, value string) error {
	if p.masterKey == "" {
		return fmt.Errorf("master key required for encrypted-file provider (set WTB_MASTER_KEY)")
	}

	// Ensure secrets directory exists
	if err := os.MkdirAll(p.secretsDir, 0700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	encrypted, err := encrypt([]byte(value), p.masterKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential %q: %w", name, err)
	}

	filePath := p.credentialPath(name)
	if err := os.WriteFile(filePath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write credential file: %w", err)
	}

	return nil
}

// Available returns true if a master key is configured.
func (p *EncryptedFileProvider) Available() bool {
	return p.masterKey != ""
}

// credentialPath returns the file path for a credential.
// Uses SHA-256 hash of the name to avoid filesystem-unsafe characters.
func (p *EncryptedFileProvider) credentialPath(name string) string {
	hash := sha256.Sum256([]byte(name))
	filename := fmt.Sprintf("%x.enc", hash)
	return filepath.Join(p.secretsDir, filename)
}

// --- Self-contained AES-256-GCM encryption (no external deps) ---
// Matches the format used by pkg/security/encryption.go for compatibility.

const pbkdf2Iterations = 100000

// encrypt encrypts data with AES-256-GCM using a password-derived key.
// Format: base64( salt(32) + nonce + ciphertext + tag )
func encrypt(data []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key via PBKDF2
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt (nonce + ciphertext + tag)
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Prepend salt
	result := append(salt, ciphertext...)

	// Base64 encode
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(result)))
	base64.StdEncoding.Encode(encoded, result)

	return encoded, nil
}

// decrypt decrypts data encrypted by encrypt().
func decrypt(data []byte, password string) ([]byte, error) {
	// Base64 decode
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decoded, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	decoded = decoded[:n]

	// Extract salt (first 32 bytes)
	if len(decoded) < 32 {
		return nil, fmt.Errorf("invalid data: too short")
	}
	salt := decoded[:32]
	ciphertext := decoded[32:]

	// Derive key
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password?): %w", err)
	}

	return plaintext, nil
}
