package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/scrypt"
)

// Vault handles local symmetric encryption for database rows.
type Vault struct {
	key []byte
}

// NewVault derives a 32-byte encryption key from the provided password and salt.
// The salt should be generated once per installation and stored securely.
func NewVault(password string, salt []byte) (*Vault, error) {
	// Derive key using scrypt (N=32768, r=8, p=1)
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}
	return &Vault{key: key}, nil
}

// GenerateSalt creates a 16-byte random salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// Encrypt encrypts plaintext using ChaCha20Poly1305.
func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(v.key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using ChaCha20Poly1305.
func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(v.key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encryptedMsg := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, encryptedMsg, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
