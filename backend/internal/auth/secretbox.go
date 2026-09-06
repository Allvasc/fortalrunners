package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// secretBox cifra o segredo TOTP em repouso com AES-256-GCM.
// A chave vem de MFA_ENC_KEY (base64 de 32 bytes) e é validada no config.
type secretBox struct{ aead cipher.AEAD }

func newSecretBox(keyB64 string) (secretBox, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return secretBox{}, fmt.Errorf("MFA_ENC_KEY não é base64 válido: %w", err)
	}
	if len(key) != 32 {
		return secretBox{}, fmt.Errorf("MFA_ENC_KEY precisa de 32 bytes (AES-256), tem %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return secretBox{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return secretBox{}, err
	}
	return secretBox{aead: aead}, nil
}

// seal devolve nonce||ciphertext||tag.
func (s secretBox) seal(plain string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (s secretBox) open(box []byte) (string, error) {
	ns := s.aead.NonceSize()
	if len(box) < ns {
		return "", errors.New("ciphertext curto demais")
	}
	nonce, body := box[:ns], box[ns:]
	plain, err := s.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("falha ao decifrar segredo MFA: %w", err)
	}
	return string(plain), nil
}
