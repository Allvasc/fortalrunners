// Package auth cuida de cadastro, login, refresh rotativo e 2FA.
// Fase 0: e-mail+senha (Argon2id) + JWT. OAuth Google e TOTP entram em seguida.
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
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

	ErrMFARequired    = errors.New("2FA obrigatório: verifique o código")
	ErrMFAInvalidCode = errors.New("código de verificação inválido")
	ErrMFANotPending  = errors.New("nenhuma configuração de 2FA pendente")
	ErrMFAAlreadyOn   = errors.New("2FA já está ativo nesta conta")
	ErrMFANotEnabled  = errors.New("2FA não está ativo nesta conta")
)

// Tokens é o par devolvido ao cliente.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// LoginResult carrega ou os tokens, ou um desafio de 2FA.
type LoginResult struct {
	User        User
	Tokens      Tokens
	MFARequired bool
	MFAToken    string // token curto para trocar por tokens em /v1/auth/mfa/verify
}

// Service é a fachada do módulo.
type Service struct {
	store   *store
	tokens  tokenIssuer
	box     crypto.Box
	oauth   OAuthConfig
	httpc   *http.Client
	webBase string
}

func NewService(deps Deps) (*Service, error) {
	box, err := crypto.NewBox(deps.MFAEncKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:   newStore(deps.Pool),
		tokens:  newTokenIssuer(deps.JWTSecret, deps.AccessTTL, deps.RefreshTTL),
		box:     box,
		oauth:   deps.OAuth,
		httpc:   &http.Client{Timeout: 10 * time.Second},
		webBase: deps.WebBaseURL,
	}, nil
}

// mobileScheme: deep link do app (app.json → "scheme").
const mobileScheme = "fortalrunners"

// RedirectURL monta o destino do callback OAuth com os tokens no fragmento
// (nunca na query — fragmento não vai para logs de servidor).
//   - dest "mobile" → volta pro app via deep link
//   - senão → portal web ("" quando não há front configurado; o handler devolve JSON)
func (s *Service) RedirectURL(t Tokens, dest string) string {
	frag := "#access_token=" + url.QueryEscape(t.AccessToken) +
		"&refresh_token=" + url.QueryEscape(t.RefreshToken) +
		"&expires_at=" + url.QueryEscape(t.ExpiresAt.Format(time.RFC3339))
	if dest == "mobile" {
		return mobileScheme + "://auth/callback" + frag
	}
	if s.webBase == "" {
		return ""
	}
	return s.webBase + "/auth/callback" + frag
}

// WebRedirectURL: compat — mesma coisa que RedirectURL(t, "").
func (s *Service) WebRedirectURL(t Tokens) string { return s.RedirectURL(t, "") }

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

	// Conta recém-criada não tem 2FA: a sessão já nasce "verificada".
	tk, err := s.issue(ctx, created, "", ip, ua, true)
	return created, tk, err
}

// Login valida credenciais. Se a conta tem 2FA ativo, devolve um desafio em vez
// dos tokens (MFARequired = true).
func (s *Service) Login(ctx context.Context, email, password, ip, ua string) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.store.userByEmail(ctx, email)
	if errors.Is(err, errNotFound) {
		return LoginResult{}, ErrInvalidCreds
	}
	if err != nil {
		return LoginResult{}, err
	}
	if u.PasswordHash == nil || !verifyPassword(password, *u.PasswordHash) {
		return LoginResult{}, ErrInvalidCreds
	}
	if u.Status != "active" {
		return LoginResult{}, ErrAccountBlocked
	}

	_, activatedAt, err := s.store.mfaState(ctx, u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if activatedAt != nil {
		ch, err := s.tokens.challenge(u.ID)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{MFARequired: true, MFAToken: ch}, nil
	}

	// Conta privilegiada sem 2FA: entra, mas a sessão NÃO é "verificada" —
	// fica sem acesso a rotas privilegiadas até ativar o 2FA (plano §13).
	tk, err := s.issue(ctx, u, "", ip, ua, !privilegedNoMFA(u.Role, activatedAt))
	return LoginResult{User: u, Tokens: tk}, err
}

// privilegedNoMFA: role admin/moderator/organizer que ainda não ativou o TOTP.
func privilegedNoMFA(role string, totpActivatedAt *time.Time) bool {
	if totpActivatedAt != nil {
		return false
	}
	switch role {
	case "admin", "moderator", "organizer":
		return true
	}
	return false
}

// VerifyMFA troca o token de desafio + código (TOTP ou recuperação) pelos tokens.
func (s *Service) VerifyMFA(ctx context.Context, challengeToken, code, ip, ua string) (User, Tokens, error) {
	userID, err := s.tokens.parseChallenge(challengeToken)
	if err != nil {
		return User{}, Tokens{}, ErrSessionInvalid
	}
	u, err := s.store.userByID(ctx, userID)
	if err != nil {
		return User{}, Tokens{}, ErrSessionInvalid
	}
	if u.Status != "active" {
		return User{}, Tokens{}, ErrAccountBlocked
	}

	ok, err := s.checkSecondFactor(ctx, userID, code)
	if err != nil {
		return User{}, Tokens{}, err
	}
	if !ok {
		return User{}, Tokens{}, ErrMFAInvalidCode
	}

	tk, err := s.issue(ctx, u, "", ip, ua, true)
	return u, tk, err
}

// checkSecondFactor aceita um código TOTP válido ou consome um código de
// recuperação de uso único.
func (s *Service) checkSecondFactor(ctx context.Context, userID, code string) (bool, error) {
	enc, activatedAt, err := s.store.mfaState(ctx, userID)
	if err != nil {
		return false, err
	}
	if activatedAt == nil || len(enc) == 0 {
		return false, ErrMFANotEnabled
	}
	secret, err := s.box.Open(enc)
	if err != nil {
		return false, err
	}
	if verifyTOTP(secret, code, time.Now()) {
		return true, nil
	}
	return s.store.consumeRecoveryCode(ctx, userID, hashRecoveryCode(code))
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
	// O refresh preserva o estado de 2FA da sessão, mas rebaixa se a conta virou
	// privilegiada e ainda não tem 2FA (ex.: runner promovido a moderator).
	verified := sess.MFAVerified
	if verified {
		if _, activatedAt, e := s.store.mfaState(ctx, u.ID); e == nil && privilegedNoMFA(u.Role, activatedAt) {
			verified = false
		}
	}
	return s.issue(ctx, u, sess.FamilyID, ip, ua, verified)
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

func (s *Service) issue(ctx context.Context, u User, familyID, ip, ua string, mfaVerified bool) (Tokens, error) {
	access, exp, err := s.tokens.access(u.ID, u.Role, mfaVerified)
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
		MFAVerified:      mfaVerified,
	}
	if err := s.store.createSession(ctx, sess, ip, ua); err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, RefreshToken: rawRefresh, ExpiresAt: exp}, nil
}

// --- 2FA: enrollment e gestão (rotas autenticadas) ---

// MFAStatusView é o que /v1/auth/mfa devolve.
type MFAStatusView struct {
	Enabled           bool       `json:"enabled"`
	Pending           bool       `json:"pending"` // segredo gerado, falta ativar
	ActivatedAt       *time.Time `json:"activated_at,omitempty"`
	RecoveryCodesLeft int        `json:"recovery_codes_left"`
}

func (s *Service) MFAStatus(ctx context.Context, userID string) (MFAStatusView, error) {
	enc, activatedAt, err := s.store.mfaState(ctx, userID)
	if err != nil {
		return MFAStatusView{}, err
	}
	v := MFAStatusView{Enabled: activatedAt != nil, ActivatedAt: activatedAt}
	v.Pending = activatedAt == nil && len(enc) > 0
	if v.Enabled {
		if n, err := s.store.countUnusedRecoveryCodes(ctx, userID); err == nil {
			v.RecoveryCodesLeft = n
		}
	}
	return v, nil
}

// SetupTOTP gera um segredo novo (pendente) e devolve o otpauth:// para o QR.
// Recusa se o 2FA já estiver ativo — desligue antes de refazer.
func (s *Service) SetupTOTP(ctx context.Context, userID string) (secret, otpauth string, err error) {
	u, err := s.store.userByID(ctx, userID)
	if err != nil {
		return "", "", ErrSessionInvalid
	}
	_, activatedAt, err := s.store.mfaState(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if activatedAt != nil {
		return "", "", ErrMFAAlreadyOn
	}

	secret, err = newTOTPSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := s.box.Seal(secret)
	if err != nil {
		return "", "", err
	}
	if err := s.store.setPendingTOTP(ctx, userID, enc); err != nil {
		return "", "", err
	}
	return secret, otpauthURL(secret, u.Email, "FortalRunners"), nil
}

// ActivateTOTP confere o primeiro código e liga o 2FA, devolvendo os códigos
// de recuperação (mostrados uma única vez).
func (s *Service) ActivateTOTP(ctx context.Context, userID, code string) ([]string, error) {
	enc, activatedAt, err := s.store.mfaState(ctx, userID)
	if err != nil {
		return nil, err
	}
	if activatedAt != nil {
		return nil, ErrMFAAlreadyOn
	}
	if len(enc) == 0 {
		return nil, ErrMFANotPending
	}
	secret, err := s.box.Open(enc)
	if err != nil {
		return nil, err
	}
	if !verifyTOTP(secret, code, time.Now()) {
		return nil, ErrMFAInvalidCode
	}

	plain, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := s.store.activateTOTP(ctx, userID, hashes); err != nil {
		return nil, err
	}
	return plain, nil
}

// DisableTOTP desliga o 2FA após conferir um código válido.
func (s *Service) DisableTOTP(ctx context.Context, userID, code string) error {
	_, activatedAt, err := s.store.mfaState(ctx, userID)
	if err != nil {
		return err
	}
	if activatedAt == nil {
		return ErrMFANotEnabled
	}
	ok, err := s.checkSecondFactor(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMFAInvalidCode
	}
	return s.store.disableTOTP(ctx, userID)
}

func isUnique(err error, constraint string) bool {
	return err != nil && strings.Contains(err.Error(), constraint)
}
