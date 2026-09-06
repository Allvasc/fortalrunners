package auth

import (
	"strings"
	"testing"
	"time"
)

func testSvc() *Service {
	return &Service{
		tokens: newTokenIssuer("segredo-de-teste-com-32-bytes-ok!!", time.Minute, time.Hour),
		oauth: OAuthConfig{
			GoogleClientID:    "gid.apps.googleusercontent.com",
			GoogleRedirectURL: "https://fortalrunners.com/v1/auth/oauth/google/callback",
		},
	}
}

func TestOAuthStateRoundtrip(t *testing.T) {
	s := testSvc()
	raw, err := s.signState("link", "user_7", "")
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.parseState(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if st.Mode != "link" || st.UID != "user_7" {
		t.Fatalf("state = %+v", st)
	}
	// adulterado não passa
	if _, err := s.parseState(raw + "x"); err == nil {
		t.Fatal("state adulterado foi aceito")
	}
	// assinatura de outro segredo não passa
	other := &Service{tokens: newTokenIssuer("outro-segredo-de-32-bytes-aqui!!!", time.Minute, time.Hour)}
	if _, err := other.parseState(raw); err == nil {
		t.Fatal("state de outro segredo foi aceito")
	}
}

func TestOAuthStartUnconfigured(t *testing.T) {
	s := testSvc()
	if _, err := s.OAuthStart("apple", "login", "", ""); err != ErrOAuthProvider {
		t.Fatalf("apple sem config deveria dar ErrOAuthProvider, deu %v", err)
	}
	if _, err := s.OAuthStart("myspace", "login", "", ""); err != ErrOAuthProvider {
		t.Fatal("provedor desconhecido")
	}
}

func TestGoogleAuthURL(t *testing.T) {
	s := testSvc()
	u, err := s.OAuthStart("google", "login", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"accounts.google.com/o/oauth2/v2/auth",
		"client_id=gid.apps.googleusercontent.com",
		"scope=openid+email+profile",
		"state=",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("authURL sem %q: %s", want, u)
		}
	}
}

func TestSanitizeUsername(t *testing.T) {
	cases := map[string]string{
		"Maria.Silva":                           "maria_silva",
		"jose+tag":                              "josetag", // '+' cai fora do alfabeto
		"a":                                     "runner",
		"...":                                   "runner",
		"MUITO_LONGO_NOME_DE_USUARIO_QUE_PASSA": "muito_longo_nome_de_usua", // 24 chars
	}
	for in, want := range cases {
		if got := sanitizeUsername(in); got != want {
			t.Errorf("sanitizeUsername(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestDecodeJWTPayload(t *testing.T) {
	// header.payload.sig — payload = {"sub":"abc","email":"x@y.z","email_verified":true}
	tok := "e30." +
		"eyJzdWIiOiJhYmMiLCJlbWFpbCI6InhAeS56IiwiZW1haWxfdmVyaWZpZWQiOnRydWV9." +
		"sig"
	var c struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := decodeJWTPayload(tok, &c); err != nil {
		t.Fatal(err)
	}
	if c.Sub != "abc" || c.Email != "x@y.z" || !c.EmailVerified {
		t.Fatalf("%+v", c)
	}
}
