package auth

import (
	"testing"
	"time"
)

func TestPasswordHashRoundtrip(t *testing.T) {
	h, err := hashPassword("uma-senha-boa-123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !verifyPassword("uma-senha-boa-123", h) {
		t.Fatal("senha correta rejeitada")
	}
	if verifyPassword("senha-errada", h) {
		t.Fatal("senha errada aceita")
	}
}

func TestAccessTokenRoundtrip(t *testing.T) {
	ti := newTokenIssuer("segredo-de-teste-com-32-bytes-ok!!", time.Minute, time.Hour)

	tok, exp, err := ti.access("user_123", "runner", true)
	if err != nil {
		t.Fatalf("emitir: %v", err)
	}
	if time.Until(exp) <= 0 {
		t.Fatal("token já expirado")
	}

	claims, err := ti.parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != "user_123" {
		t.Fatalf("sub = %q, esperado user_123", claims.Subject)
	}
	if claims.Role != "runner" {
		t.Fatalf("role = %q, esperado runner", claims.Role)
	}
	if !claims.MFA {
		t.Fatal("claim mfa deveria ser true")
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	a := newTokenIssuer("segredo-a-com-32-bytes-de-verdade!", time.Minute, time.Hour)
	b := newTokenIssuer("segredo-b-com-32-bytes-de-verdade!", time.Minute, time.Hour)

	tok, _, _ := a.access("u", "runner", false)
	if _, err := b.parse(tok); err == nil {
		t.Fatal("token com assinatura de outro segredo foi aceito")
	}
}

func TestRefreshTokenHashDeterministic(t *testing.T) {
	raw, h := newRefreshToken()
	if raw == "" || h == "" {
		t.Fatal("token/hash vazios")
	}
	if hashRefreshToken(raw) != h {
		t.Fatal("hash do refresh não é determinístico")
	}
}
