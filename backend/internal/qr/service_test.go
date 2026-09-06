package qr

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

// mintPayload replica o formato emitido por GetToken sem tocar no banco:
//   fr1.<base64url(user_id|exp_unix|nonce)>.<hmac_hex>
func (s *Service) mintPayload(userID string, exp time.Time) string {
	body := base64.RawURLEncoding.EncodeToString([]byte(
		userID + "|" + strconv.FormatInt(exp.Unix(), 10) + "|" + base64.RawURLEncoding.EncodeToString([]byte("nonce1234")),
	))
	return "fr1." + body + "." + s.sign(body)
}

func TestVerify_RoundTrip(t *testing.T) {
	s := &Service{secret: []byte("segredo-de-teste")}
	p := s.mintPayload("usr_abc", time.Now().Add(30*time.Second))

	uid, err := s.verify(p)
	if err != nil {
		t.Fatalf("payload válido rejeitado: %v", err)
	}
	if uid != "usr_abc" {
		t.Fatalf("user_id errado: %q", uid)
	}
}

func TestVerify_TamperedSignature(t *testing.T) {
	s := &Service{secret: []byte("segredo-de-teste")}
	p := s.mintPayload("usr_abc", time.Now().Add(30*time.Second))
	// vira o último caractere da assinatura
	b := []byte(p)
	if b[len(b)-1] == 'a' {
		b[len(b)-1] = 'b'
	} else {
		b[len(b)-1] = 'a'
	}
	if _, err := s.verify(string(b)); err == nil {
		t.Fatal("assinatura adulterada foi aceita")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	issuer := &Service{secret: []byte("segredo-A")}
	attacker := &Service{secret: []byte("segredo-B")}
	p := issuer.mintPayload("usr_abc", time.Now().Add(30*time.Second))
	if _, err := attacker.verify(p); err == nil {
		t.Fatal("token assinado com outro segredo foi aceito")
	}
}

func TestVerify_Expired(t *testing.T) {
	s := &Service{secret: []byte("segredo-de-teste")}
	p := s.mintPayload("usr_abc", time.Now().Add(-1*time.Second))
	if _, err := s.verify(p); err == nil {
		t.Fatal("token expirado foi aceito")
	}
}

func TestVerify_Malformed(t *testing.T) {
	s := &Service{secret: []byte("x")}
	for _, bad := range []string{"", "fr1", "fr1.abc", "xx.yy.zz", "fr1..", "fr1.notbase64!.sig"} {
		if _, err := s.verify(bad); err == nil {
			t.Fatalf("payload malformado aceito: %q", bad)
		}
	}
}
