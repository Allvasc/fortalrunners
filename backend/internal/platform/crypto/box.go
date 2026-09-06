// Package crypto guarda a primitiva de criptografia de campo (AES-256-GCM) usada
// para cifrar segredos em repouso: o segredo TOTP e os tokens de integrações.
// A chave vem do cofre (base64 de 32 bytes) e é validada no config.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Box cifra e decifra bytes com AES-256-GCM (nonce aleatório prefixado).
type Box struct{ aead cipher.AEAD }

// NewBox monta o cofre a partir de uma chave base64 de exatamente 32 bytes.
func NewBox(keyB64 string) (Box, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return Box{}, fmt.Errorf("chave de criptografia não é base64 válido: %w", err)
	}
	if len(key) != 32 {
		return Box{}, fmt.Errorf("chave de criptografia precisa de 32 bytes (AES-256), tem %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Box{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Box{}, err
	}
	return Box{aead: aead}, nil
}

// Seal devolve nonce||ciphertext||tag.
func (b Box) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Open reverte Seal.
func (b Box) Open(box []byte) (string, error) {
	ns := b.aead.NonceSize()
	if len(box) < ns {
		return "", errors.New("ciphertext curto demais")
	}
	nonce, body := box[:ns], box[ns:]
	plain, err := b.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("falha ao decifrar: %w", err)
	}
	return string(plain), nil
}
