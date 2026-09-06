package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims do access token.
type Claims struct {
	Role string `json:"role"`
	// MFA indica que a sessão passou por 2FA (ou que a conta não tem 2FA ativo).
	// Rotas privilegiadas exigem MFA == true.
	MFA bool `json:"mfa"`
	jwt.RegisteredClaims
}

// challengeClaims é o token curto emitido entre senha correta e código TOTP.
type challengeClaims struct {
	Typ string `json:"typ"` // sempre "mfa_challenge"
	jwt.RegisteredClaims
}

const challengeTTL = 5 * time.Minute

type tokenIssuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func newTokenIssuer(secret string, accessTTL, refreshTTL time.Duration) tokenIssuer {
	return tokenIssuer{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// access gera um JWT HS256 curto com sub = userID.
func (t tokenIssuer) access(userID, role string, mfa bool) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(t.accessTTL)
	c := Claims{
		Role: role,
		MFA:  mfa,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "fortalrunners",
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	return s, exp, err
}

// challenge emite o token intermediário do fluxo de 2FA.
func (t tokenIssuer) challenge(userID string) (string, error) {
	now := time.Now()
	c := challengeClaims{
		Typ: "mfa_challenge",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(challengeTTL)),
			Issuer:    "fortalrunners",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
}

// parseChallenge valida o token intermediário e devolve o userID.
func (t tokenIssuer) parseChallenge(token string) (string, error) {
	var c challengeClaims
	_, err := jwt.ParseWithClaims(token, &c, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("assinatura inesperada: %v", tok.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", err
	}
	if c.Typ != "mfa_challenge" || c.Subject == "" {
		return "", fmt.Errorf("token de desafio inválido")
	}
	return c.Subject, nil
}

func (t tokenIssuer) parse(token string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("assinatura inesperada: %v", tok.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// newRefreshToken devolve o token opaco (mandado ao cliente) e o hash (guardado no banco).
func newRefreshToken() (token string, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(sum[:])
	return
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
