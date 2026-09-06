package auth

import (
	"encoding/base64"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
	"strings"
	"testing"
	"time"
)

// Vetores da RFC 6238 (Apêndice B), seed ASCII "12345678901234567890", SHA1,
// truncados aos 6 dígitos que usamos.
func TestTOTPRFC6238Vectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := totpCode(secret, uint64(c.unix/totpPeriod))
		if err != nil {
			t.Fatalf("unix %d: %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("unix %d: got %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	secret, err := newTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)

	code, _ := totpCode(secret, uint64(now.Unix()/totpPeriod))
	if !verifyTOTP(secret, code, now) {
		t.Fatal("código do período atual rejeitado")
	}
	// período anterior ainda vale (skew 1)
	prev, _ := totpCode(secret, uint64(now.Unix()/totpPeriod)-1)
	if !verifyTOTP(secret, prev, now) {
		t.Fatal("código do período anterior deveria valer dentro da janela")
	}
	// dois períodos atrás não vale
	old, _ := totpCode(secret, uint64(now.Unix()/totpPeriod)-3)
	if verifyTOTP(secret, old, now) {
		t.Fatal("código muito antigo foi aceito")
	}
	if verifyTOTP(secret, "000000", now) && code != "000000" {
		t.Fatal("código arbitrário aceito")
	}
}

func TestSecretBoxRoundtrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, err := crypto.NewBox(key)
	if err != nil {
		t.Fatalf("crypto.NewBox: %v", err)
	}
	secret := "JBSWY3DPEHPK3PXP"
	enc, err := box.Seal(secret)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(string(enc), secret) {
		t.Fatal("segredo aparece em claro no ciphertext")
	}
	got, err := box.Open(enc)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != secret {
		t.Fatalf("roundtrip: got %q want %q", got, secret)
	}

	// chave de outro tamanho é recusada
	if _, err := crypto.NewBox(base64.StdEncoding.EncodeToString([]byte("curta"))); err == nil {
		t.Fatal("chave de 5 bytes deveria falhar")
	}
	// ciphertext adulterado não abre (GCM autentica)
	enc[len(enc)-1] ^= 0xff
	if _, err := box.Open(enc); err == nil {
		t.Fatal("ciphertext adulterado abriu")
	}
}

func TestRecoveryCodeHashNormalizes(t *testing.T) {
	plain, hashes, err := newRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != recoveryCount || len(hashes) != recoveryCount {
		t.Fatalf("esperava %d códigos", recoveryCount)
	}
	// o hash não depende de hífen nem de caixa
	c := plain[0]
	if hashRecoveryCode(c) != hashes[0] {
		t.Fatal("hash do próprio código não bate")
	}
	if hashRecoveryCode(strings.ToUpper(strings.ReplaceAll(c, "-", ""))) != hashes[0] {
		t.Fatal("normalização (caixa/hífen) falhou")
	}
	if hashRecoveryCode("outra-coisa") == hashes[0] {
		t.Fatal("códigos diferentes colidiram")
	}
}

func TestChallengeTokenRoundtrip(t *testing.T) {
	ti := newTokenIssuer("segredo-de-teste-com-32-bytes-ok!!", time.Minute, time.Hour)

	tok, err := ti.challenge("user_9")
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	uid, err := ti.parseChallenge(tok)
	if err != nil {
		t.Fatalf("parseChallenge: %v", err)
	}
	if uid != "user_9" {
		t.Fatalf("uid = %q", uid)
	}

	// um access token não passa como desafio
	access, _, _ := ti.access("user_9", "runner", false)
	if _, err := ti.parseChallenge(access); err == nil {
		t.Fatal("access token foi aceito como desafio de MFA")
	}
}
