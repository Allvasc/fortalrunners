package auth

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrOAuthProvider  = errors.New("provedor OAuth desconhecido ou não configurado")
	ErrOAuthState     = errors.New("state OAuth inválido ou expirado")
	ErrOAuthExchange  = errors.New("falha ao trocar o código OAuth")
	ErrIdentityLinked = errors.New("essa conta social já está ligada a outro usuário")
)

// oauthUser é o que extraímos do provedor após o exchange.
type oauthUser struct {
	ProviderUID   string
	Email         string
	EmailVerified bool
	Name          string
}

type oauthProvider interface {
	name() string
	authURL(state string) string
	exchange(ctx context.Context, code string) (oauthUser, error)
}

// OAuthConfig é o que o app injeta.
type OAuthConfig struct {
	GoogleClientID, GoogleClientSecret, GoogleRedirectURL                string
	AppleClientID, AppleTeamID, AppleKeyID, AppleP8Key, AppleRedirectURL string
}

func (s *Service) provider(name string) (oauthProvider, bool) {
	switch name {
	case "google":
		if s.oauth.GoogleClientID == "" {
			return nil, false
		}
		return googleProvider{s.oauth, s.httpc}, true
	case "apple":
		if s.oauth.AppleClientID == "" || s.oauth.AppleP8Key == "" {
			return nil, false
		}
		return appleProvider{s.oauth, s.httpc}, true
	}
	return nil, false
}

// --- state assinado (anti-CSRF) ---

type oauthState struct {
	Mode string `json:"m"` // login | link
	UID  string `json:"u,omitempty"`
	Dest string `json:"d,omitempty"` // "mobile" → volta pro app; senão pro portal web
	jwt.RegisteredClaims
}

func (s *Service) signState(mode, uid, dest string) (string, error) {
	c := oauthState{Mode: mode, UID: uid, Dest: dest, RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		Issuer:    "fortalrunners-oauth",
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.tokens.secret)
}

func (s *Service) parseState(raw string) (oauthState, error) {
	var c oauthState
	_, err := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		return s.tokens.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("fortalrunners-oauth"))
	if err != nil {
		return c, ErrOAuthState
	}
	return c, nil
}

// --- fluxo público ---

// OAuthStart devolve a URL de autorização do provedor. mode: "login" ou "link".
// dest: "mobile" faz o callback voltar pro app; qualquer outra coisa → portal web.
func (s *Service) OAuthStart(provider, mode, uid, dest string) (string, error) {
	p, ok := s.provider(provider)
	if !ok {
		return "", ErrOAuthProvider
	}
	if mode != "link" {
		mode = "login"
	}
	if dest != "mobile" {
		dest = ""
	}
	state, err := s.signState(mode, uid, dest)
	if err != nil {
		return "", err
	}
	return p.authURL(state), nil
}

// OAuthCallback finaliza o login/vínculo social. O 3º retorno é o `dest` do state.
func (s *Service) OAuthCallback(ctx context.Context, provider, code, rawState, ip, ua string) (User, Tokens, string, error) {
	p, ok := s.provider(provider)
	if !ok {
		return User{}, Tokens{}, "", ErrOAuthProvider
	}
	st, err := s.parseState(rawState)
	if err != nil {
		return User{}, Tokens{}, "", err
	}
	ou, err := p.exchange(ctx, code)
	if err != nil {
		return User{}, Tokens{}, "", ErrOAuthExchange
	}
	if ou.ProviderUID == "" {
		return User{}, Tokens{}, "", ErrOAuthExchange
	}

	u, err := s.resolveOAuth(ctx, provider, st, ou)
	if err != nil {
		return User{}, Tokens{}, "", err
	}
	if u.Status != "active" {
		return User{}, Tokens{}, "", ErrAccountBlocked
	}
	// Conta social + role privilegiado sem 2FA: sessão não-verificada (plano §13).
	tk, err := s.issue(ctx, u, "", ip, ua, !privilegedNoMFA(u.Role, nil) || func() bool {
		_, act, _ := s.store.mfaState(ctx, u.ID)
		return act != nil
	}())
	return u, tk, st.Dest, err
}

// resolveOAuth: identidade existente → login; e-mail conhecido → vincula; novo → cria.
func (s *Service) resolveOAuth(ctx context.Context, provider string, st oauthState, ou oauthUser) (User, error) {
	if uid, err := s.store.userIDByIdentity(ctx, provider, ou.ProviderUID); err == nil {
		if st.Mode == "link" && st.UID != "" && st.UID != uid {
			return User{}, ErrIdentityLinked
		}
		return s.store.userByID(ctx, uid)
	} else if !errors.Is(err, errNotFound) {
		return User{}, err
	}

	if st.Mode == "link" && st.UID != "" {
		if err := s.store.linkIdentity(ctx, id.New(), st.UID, provider, ou.ProviderUID, ou.Email); err != nil {
			return User{}, err
		}
		return s.store.userByID(ctx, st.UID)
	}

	email := strings.ToLower(strings.TrimSpace(ou.Email))
	if email != "" {
		if u, err := s.store.userByEmail(ctx, email); err == nil {
			_ = s.store.linkIdentity(ctx, id.New(), u.ID, provider, ou.ProviderUID, ou.Email)
			return u, nil
		} else if !errors.Is(err, errNotFound) {
			return User{}, err
		}
	}

	// cria conta nova
	u := User{ID: id.New(), Username: s.freeUsername(ctx, email), Email: email}
	if ou.Name != "" {
		u.DisplayName = &ou.Name
	}
	created, err := s.store.createOAuthUser(ctx, u, provider, ou.ProviderUID, ou.EmailVerified)
	return created, err
}

func (s *Service) freeUsername(ctx context.Context, email string) string {
	base := "runner"
	if i := strings.IndexByte(email, '@'); i > 0 {
		base = sanitizeUsername(email[:i])
	}
	for try := 0; try < 6; try++ {
		cand := base
		if try > 0 {
			cand = fmt.Sprintf("%s%d", base, 100+randInt(9900))
		}
		if free, _ := s.store.usernameFree(ctx, cand); free {
			return cand
		}
	}
	return fmt.Sprintf("runner%d", randInt(1_000_000))
}

// --- Google ---

type googleProvider struct {
	cfg   OAuthConfig
	httpc *http.Client
}

func (googleProvider) name() string { return "google" }

func (g googleProvider) authURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.cfg.GoogleClientID)
	q.Set("redirect_uri", g.cfg.GoogleRedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

func (g googleProvider) exchange(ctx context.Context, code string) (oauthUser, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.cfg.GoogleClientID)
	form.Set("client_secret", g.cfg.GoogleClientSecret)
	form.Set("redirect_uri", g.cfg.GoogleRedirectURL)
	form.Set("grant_type", "authorization_code")

	idToken, err := postForm(ctx, g.httpc, "https://oauth2.googleapis.com/token", form)
	if err != nil {
		return oauthUser{}, err
	}
	var c struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := decodeJWTPayload(idToken, &c); err != nil {
		return oauthUser{}, err
	}
	return oauthUser{ProviderUID: c.Sub, Email: c.Email, EmailVerified: c.EmailVerified, Name: c.Name}, nil
}

// --- Apple ---

type appleProvider struct {
	cfg   OAuthConfig
	httpc *http.Client
}

func (appleProvider) name() string { return "apple" }

func (a appleProvider) authURL(state string) string {
	q := url.Values{}
	q.Set("client_id", a.cfg.AppleClientID)
	q.Set("redirect_uri", a.cfg.AppleRedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "name email")
	q.Set("response_mode", "form_post")
	q.Set("state", state)
	return "https://appleid.apple.com/auth/authorize?" + q.Encode()
}

func (a appleProvider) exchange(ctx context.Context, code string) (oauthUser, error) {
	secret, err := a.clientSecret()
	if err != nil {
		return oauthUser{}, err
	}
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", a.cfg.AppleClientID)
	form.Set("client_secret", secret)
	form.Set("redirect_uri", a.cfg.AppleRedirectURL)
	form.Set("grant_type", "authorization_code")

	idToken, err := postForm(ctx, a.httpc, "https://appleid.apple.com/auth/token", form)
	if err != nil {
		return oauthUser{}, err
	}
	var c struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"` // Apple manda "true" (string) ou bool
	}
	if err := decodeJWTPayload(idToken, &c); err != nil {
		return oauthUser{}, err
	}
	verified := c.EmailVerified == true || c.EmailVerified == "true"
	return oauthUser{ProviderUID: c.Sub, Email: c.Email, EmailVerified: verified}, nil
}

// clientSecret assina o JWT ES256 que a Apple exige como client_secret.
func (a appleProvider) clientSecret() (string, error) {
	block, _ := pem.Decode([]byte(a.cfg.AppleP8Key))
	if block == nil {
		return "", errors.New("APPLE_P8_KEY não é PEM válido")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    a.cfg.AppleTeamID,
		Subject:   a.cfg.AppleClientID,
		Audience:  jwt.ClaimStrings{"https://appleid.apple.com"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Minute)),
	})
	t.Header["kid"] = a.cfg.AppleKeyID
	return t.SignedString(key)
}

// --- helpers HTTP/JWT ---

func postForm(ctx context.Context, hc *http.Client, endpoint string, form url.Values) (idToken string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.IDToken == "" {
		return "", errors.New("resposta sem id_token")
	}
	return out.IDToken, nil
}

// decodeJWTPayload lê o corpo do JWT sem verificar assinatura — seguro só porque
// o token veio direto do token endpoint do provedor, sobre TLS.
func decodeJWTPayload(token string, v any) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("JWT malformado")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func sanitizeUsername(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) < 3 {
		return "runner"
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func randInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}
