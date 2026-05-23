package gdrive

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// deriveKey returns a 32-byte AES key derived from the machine ID.
func deriveKey() []byte {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		data = []byte("styx-fallback-key")
	}
	hash := sha256.Sum256(data)
	return hash[:]
}

// encrypt encrypts plaintext using AES-256-GCM and returns the ciphertext
// prefixed with the 12-byte nonce.
func encrypt(plaintext []byte) ([]byte, error) {
	key := deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts ciphertext (nonce prefixed) using AES-256-GCM.
func decrypt(ciphertext []byte) ([]byte, error) {
	key := deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
