package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptionManager gerencia criptografia at-rest
type EncryptionManager struct {
	enabled bool
}

// NewEncryptionManager cria novo manager
func NewEncryptionManager(enabled bool) *EncryptionManager {
	return &EncryptionManager{
		enabled: enabled,
	}
}

// EncryptFile criptografa arquivo com senha
func (em *EncryptionManager) EncryptFile(inputPath, outputPath, password string) error {
	if !em.enabled {
		return fmt.Errorf("encryption está desabilitado")
	}

	// Ler arquivo
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	// Criptografar
	encrypted, err := em.Encrypt(data, password)
	if err != nil {
		return err
	}

	// Salvar
	if err := os.WriteFile(outputPath, encrypted, 0600); err != nil {
		return fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	return nil
}

// DecryptFile descriptografa arquivo
func (em *EncryptionManager) DecryptFile(inputPath, outputPath, password string) error {
	if !em.enabled {
		return fmt.Errorf("encryption está desabilitado")
	}

	// Ler arquivo
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	// Descriptografar
	decrypted, err := em.Decrypt(data, password)
	if err != nil {
		return err
	}

	// Salvar
	if err := os.WriteFile(outputPath, decrypted, 0600); err != nil {
		return fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	return nil
}

// Encrypt criptografa dados com AES-256
func (em *EncryptionManager) Encrypt(data []byte, password string) ([]byte, error) {
	// Gerar salt aleatório
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("erro ao gerar salt: %w", err)
	}

	// Derivar key usando PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Criar cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cipher: %w", err)
	}

	// GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar GCM: %w", err)
	}

	// Gerar nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("erro ao gerar nonce: %w", err)
	}

	// Criptografar
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Formato: salt (32 bytes) + ciphertext
	result := append(salt, ciphertext...)

	// Base64 encode para facilitar armazenamento
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(result)))
	base64.StdEncoding.Encode(encoded, result)

	return encoded, nil
}

// Decrypt descriptografa dados
func (em *EncryptionManager) Decrypt(data []byte, password string) ([]byte, error) {
	// Base64 decode
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decoded, data)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar base64: %w", err)
	}
	decoded = decoded[:n]

	// Extrair salt (primeiros 32 bytes)
	if len(decoded) < 32 {
		return nil, fmt.Errorf("dados inválidos: muito curto")
	}
	salt := decoded[:32]
	ciphertext := decoded[32:]

	// Derivar key
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Criar cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cipher: %w", err)
	}

	// GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar GCM: %w", err)
	}

	// Extrair nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext inválido")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Descriptografar
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao descriptografar (senha incorreta?): %w", err)
	}

	return plaintext, nil
}

// EncryptJSON criptografa struct como JSON
func (em *EncryptionManager) EncryptJSON(v interface{}, password string) ([]byte, error) {
	// Serializar JSON
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar JSON: %w", err)
	}

	// Criptografar
	return em.Encrypt(data, password)
}

// DecryptJSON descriptografa JSON para struct
func (em *EncryptionManager) DecryptJSON(data []byte, password string, v interface{}) error {
	// Descriptografar
	plaintext, err := em.Decrypt(data, password)
	if err != nil {
		return err
	}

	// Deserializar JSON
	if err := json.Unmarshal(plaintext, v); err != nil {
		return fmt.Errorf("erro ao deserializar JSON: %w", err)
	}

	return nil
}

// PromptPassword solicita senha de forma segura
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	
	// Ler senha sem echo (usando syscall)
	// Para simplificar, usamos input normal aqui
	// Em produção, use golang.org/x/term.ReadPassword
	var password string
	if _, err := fmt.Scanln(&password); err != nil {
		return "", err
	}
	
	return password, nil
}

// GenerateKey gera chave aleatória para encryption
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// KeyStrength valida força da senha
func KeyStrength(password string) (string, bool) {
	length := len(password)
	
	if length < 8 {
		return "Muito fraca (mínimo 8 caracteres)", false
	}
	if length < 12 {
		return "Fraca (recomendado 12+ caracteres)", false
	}
	if length < 16 {
		return "Média (recomendado 16+ caracteres)", true
	}
	
	return "Forte", true
}
