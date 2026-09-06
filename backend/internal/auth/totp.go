package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Parâmetros TOTP — os padrões que Google Authenticator, Authy, 1Password e
// Aegis assumem quando o otpauth:// não diz o contrário.
const (
	totpPeriod      = 30 // segundos por código
	totpDigits      = 6
	totpSkew        = 1  // aceita a janela [-1, +1] períodos (relógio torto)
	totpSecretBytes = 20 // 160 bits, recomendação da RFC 4226 §4
	recoveryCount   = 10
)

// base32 sem padding, alfabeto RFC 4648 — o que os apps de autenticação leem.
var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// newTOTPSecret gera um segredo novo em base32.
func newTOTPSecret() (string, error) {
	b := make([]byte, totpSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// totpCode calcula o código para um contador de tempo (HMAC-SHA1 + truncamento
// dinâmico, RFC 4226 §5.3).
func totpCode(secret string, counter uint64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("segredo TOTP inválido: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%0*d", totpDigits, bin%pow10(totpDigits)), nil
}

// verifyTOTP confere o código na janela de tolerância, em tempo constante.
func verifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	base := now.Unix() / totpPeriod
	ok := false
	for delta := int64(-totpSkew); delta <= totpSkew; delta++ {
		want, err := totpCode(secret, uint64(base+delta))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			ok = true // não damos break: mantém o tempo previsível
		}
	}
	return ok
}

// otpauthURL monta a URI que vira QR code no app de autenticação.
func otpauthURL(secret, account, issuer string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// newRecoveryCodes gera os códigos de uso único (texto para mostrar uma vez;
// hash para guardar). Formato: 4-4 caracteres base32 minúsculos, ex. "k7f2-9qab".
func newRecoveryCodes() (plain []string, hashes []string, err error) {
	for i := 0; i < recoveryCount; i++ {
		b := make([]byte, 5) // 8 chars base32
		if _, err = rand.Read(b); err != nil {
			return nil, nil, err
		}
		s := strings.ToLower(b32.EncodeToString(b))
		code := s[:4] + "-" + s[4:8]
		plain = append(plain, code)
		hashes = append(hashes, hashRecoveryCode(code))
	}
	return plain, hashes, nil
}

// hashRecoveryCode normaliza (sem hífen, minúsculo) e devolve o SHA-256 hex.
// Reusa hashRefreshToken (sha256) para não duplicar primitiva.
func hashRecoveryCode(code string) string {
	norm := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	return hashRefreshToken(norm)
}

func pow10(n int) uint32 {
	p := uint32(1)
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}
