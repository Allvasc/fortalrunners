// Package auth cuida de cadastro, login, refresh rotativo e 2FA.
// Fase 0: e-mail+senha (Argon2id) + JWT. OAuth Google e TOTP entram em seguida.
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrEmailTaken     = errors.New("e-mail já cadastrado")
	ErrUsernameTaken  = errors.New("nome de usuário já em uso")
	ErrInvalidCreds   = errors.New("e-mail ou senha inválidos")
	ErrSessionInvalid = errors.New("sessão inválida ou expirada")
	ErrTokenReused    = errors.New("refresh token reutilizado — sessões revogadas")
	ErrWeakPassword   = errors.New("a senha precisa de pelo menos 8 caracteres")
	ErrAccountBlocked = errors.New("conta suspensa ou banida")
)

// Tokens é o par devolvido ao cliente.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Service é a fachada do módulo.
type Service struct {
	store  *store
	tokens tokenIssuer
}

func NewService(deps Deps) *Service {
	return &Service{
		store:  newStore(deps.Pool),
		tokens: newTokenIssuer(deps.JWTSecret, deps.AccessTTL, deps.RefreshTTL),
	}
}

// Register cria a conta e já devolve tokens (auto-login).
func (s *Service) Register(ctx context.Context, in RegisterInput, ip, ua string) (User, Tokens, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Password) < 8 {
		return User{}, Tokens{}, ErrWeakPassword
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return User{}, Tokens{}, err
	}
	name := in.DisplayName
	u := User{ID: id.New(), Username: in.Username, Email: in.Email, PasswordHash: &hash}
	if name != "" {
		u.DisplayName = &name
	}

	created, err := s.store.createUser(ctx, u)
	if err != nil {
		switch {
		case isUnique(err, "users_email_key"):
			return User{}, Tokens{}, ErrEmailTaken
		case isUnique(err, "users_username_key"):
			return User{}, Tokens{}, ErrUsernameTaken
		default:
			return User{}, Tokens{}, err
		}
	}

	tk, err := s.issue(ctx, created, "", ip, ua)
	return created, tk, err
}

// Login valida credenciais e abre uma nova família de sessão.
func (s *Service) Login(ctx context.Context, email, password, ip, ua string) (User, Tokens, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.store.userByEmail(ctx, email)
	if errors.Is(err, errNotFound) {
		return User{}, Tokens{}, ErrInvalidCreds
	}
	if err != nil {
		return User{}, Tokens{}, err
	}
	if u.PasswordHash == nil || !verifyPassword(password, *u.PasswordHash) {
		return User{}, Tokens{}, ErrInvalidCreds
	}
	if u.Status != "active" {
		return User{}, Tokens{}, ErrAccountBlocked
	}
	tk, err := s.issue(ctx, u, "", ip, ua)
	return u, tk, err
}

// Refresh rotaciona o token. Se o apresentado já foi trocado, revoga a família inteira.
func (s *Service) Refresh(ctx context.Context, refreshToken, ip, ua string) (Tokens, error) {
	sess, err := s.store.sessionByRefreshHash(ctx, hashRefreshToken(refreshToken))
	if errors.Is(err, errNotFound) {
		return Tokens{}, ErrSessionInvalid
	}
	if err != nil {
		return Tokens{}, err
	}
	if sess.RevokedAt != nil {
		_ = s.store.revokeFamily(ctx, sess.FamilyID)
		return Tokens{}, ErrTokenReused
	}
	if time.Now().After(sess.ExpiresAt) {
		return Tokens{}, ErrSessionInvalid
	}

	u, err := s.store.userByID(ctx, sess.UserID)
	if err != nil {
		return Tokens{}, ErrSessionInvalid
	}
	if err := s.store.revokeSession(ctx, sess.ID); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, u, sess.FamilyID, ip, ua)
}

// Logout revoga a sessão do refresh apresentado.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	sess, err := s.store.sessionByRefreshHash(ctx, hashRefreshToken(refreshToken))
	if errors.Is(err, errNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.store.revokeSession(ctx, sess.ID)
}

// Me devolve o usuário do access token.
func (s *Service) Me(ctx context.Context, userID string) (User, error) {
	return s.store.userByID(ctx, userID)
}

// ParseAccess valida um access token e devolve as claims.
func (s *Service) ParseAccess(token string) (*Claims, error) {
	return s.tokens.parse(token)
}

func (s *Service) issue(ctx context.Context, u User, familyID, ip, ua string) (Tokens, error) {
	access, exp, err := s.tokens.access(u.ID, u.Role)
	if err != nil {
		return Tokens{}, err
	}
	rawRefresh, refreshHash := newRefreshToken()
	if familyID == "" {
		familyID = id.New()
	}
	sess := session{
		ID:               id.New(),
		UserID:           u.ID,
		FamilyID:         familyID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        time.Now().Add(s.tokens.refreshTTL),
	}
	if err := s.store.createSession(ctx, sess, ip, ua); err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, RefreshToken: rawRefresh, ExpiresAt: exp}, nil
}

func isUnique(err error, constraint string) bool {
	return err != nil && strings.Contains(err.Error(), constraint)
}
