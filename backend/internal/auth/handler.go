package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Deps é o que o módulo precisa de fora.
type Deps struct {
	Pool       *pgxpool.Pool
	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	MFAEncKey  string // base64 de 32 bytes (AES-256-GCM do segredo TOTP)
	OAuth      OAuthConfig
	WebBaseURL string // para onde o callback OAuth redireciona (fragmento com tokens)
	Log        *slog.Logger
	Mailer     Mailer // nil → logMailer (só registra o link no log)
}

type RegisterInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// Handler expõe as rotas de auth.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register monta o grupo público /v1/auth.
func (h *Handler) Register(g *echo.Group) {
	a := g.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.POST("/refresh", h.refresh)
	a.POST("/logout", h.logout)
	a.POST("/mfa/verify", h.mfaVerify) // troca o desafio de login pelo par de tokens
	a.POST("/password/forgot", h.passwordForgot)
	a.POST("/password/reset", h.passwordReset)
	a.POST("/email/verify", h.emailVerify) // consome o token do e-mail (público)

	a.GET("/oauth/:provider", h.oauthStart)
	a.GET("/oauth/:provider/callback", h.oauthCallback)
	a.POST("/oauth/:provider/callback", h.oauthCallback) // Apple usa form_post
}

// RegisterSecured monta as rotas de 2FA que exigem Bearer token.
func (h *Handler) RegisterSecured(g *echo.Group) {
	m := g.Group("/auth/mfa")
	m.GET("", h.mfaStatus)
	m.POST("/setup", h.mfaSetup)
	m.POST("/activate", h.mfaActivate)
	m.POST("/disable", h.mfaDisable)

	g.POST("/auth/email/verify/send", h.emailVerifySend) // reenvia o e-mail de confirmação
}

func (h *Handler) emailVerify(c echo.Context) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token obrigatório")
	}
	if err := h.svc.VerifyEmail(c.Request().Context(), req.Token); err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) emailVerifySend(c echo.Context) error {
	if err := h.svc.SendEmailVerification(c.Request().Context(), UserID(c)); err != nil {
		if errors.Is(err, ErrAlreadyVerified) {
			return c.JSON(http.StatusOK, map[string]string{"status": "already_verified"})
		}
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) passwordForgot(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	// resposta sempre igual — não revela se o e-mail existe
	h.svc.RequestPasswordReset(c.Request().Context(), req.Email)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) passwordReset(c echo.Context) error {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token e password obrigatórios")
	}
	if err := h.svc.ResetPassword(c.Request().Context(), req.Token, req.Password); err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) register(c echo.Context) error {
	var in RegisterInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	u, tk, err := h.svc.Register(c.Request().Context(), in, clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"user": publicUser(u), "tokens": tk})
}

func (h *Handler) login(c echo.Context) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	res, err := h.svc.Login(c.Request().Context(), in.Email, in.Password, clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	if res.MFARequired {
		return c.JSON(http.StatusOK, map[string]any{"mfa_required": true, "mfa_token": res.MFAToken})
	}
	return c.JSON(http.StatusOK, map[string]any{"user": publicUser(res.User), "tokens": res.Tokens})
}

func (h *Handler) mfaVerify(c echo.Context) error {
	var in struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}
	if err := c.Bind(&in); err != nil || in.MFAToken == "" || in.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "mfa_token e code são obrigatórios")
	}
	u, tk, err := h.svc.VerifyMFA(c.Request().Context(), in.MFAToken, in.Code, clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"user": publicUser(u), "tokens": tk})
}

// oauthStart redireciona o navegador para o provedor. ?mode=link (com Bearer)
// para vincular à conta logada; senão login.
func (h *Handler) oauthStart(c echo.Context) error {
	mode := c.QueryParam("mode")
	var uid string
	if mode == "link" {
		claims, err := h.svc.ParseAccess(strings.TrimPrefix(c.Request().Header.Get(echo.HeaderAuthorization), "Bearer "))
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "token necessário para vincular")
		}
		uid = claims.Subject
	}
	dest := c.QueryParam("dest") // "mobile" faz o callback voltar pro app
	url, err := h.svc.OAuthStart(c.Param("provider"), mode, uid, dest)
	if err != nil {
		return authErr(err)
	}
	// mode=link vem de um fetch com Bearer (não pode seguir 302 por CORS) → JSON.
	// dest=mobile também: o app abre a URL num browser in-app.
	if mode == "link" || dest == "mobile" || c.QueryParam("format") == "json" {
		return c.JSON(http.StatusOK, map[string]string{"authorize_url": url})
	}
	return c.Redirect(http.StatusFound, url)
}

// oauthCallback é o redirect de volta do provedor.
func (h *Handler) oauthCallback(c echo.Context) error {
	code := c.FormValue("code")
	state := c.FormValue("state")
	if code == "" || state == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code e state obrigatórios")
	}
	u, tk, dest, err := h.svc.OAuthCallback(c.Request().Context(), c.Param("provider"), code, state,
		clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	if target := h.svc.RedirectURL(tk, dest); target != "" {
		return c.Redirect(http.StatusFound, target)
	}
	return c.JSON(http.StatusOK, map[string]any{"user": publicUser(u), "tokens": tk})
}

// --- 2FA: enrollment (autenticado) ---

func (h *Handler) mfaStatus(c echo.Context) error {
	v, err := h.svc.MFAStatus(c.Request().Context(), UserID(c))
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, v)
}

func (h *Handler) mfaSetup(c echo.Context) error {
	secret, otpauth, err := h.svc.SetupTOTP(c.Request().Context(), UserID(c))
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"secret": secret, "otpauth_url": otpauth})
}

func (h *Handler) mfaActivate(c echo.Context) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&in); err != nil || in.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code é obrigatório")
	}
	codes, err := h.svc.ActivateTOTP(c.Request().Context(), UserID(c), in.Code)
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (h *Handler) mfaDisable(c echo.Context) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&in); err != nil || in.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code é obrigatório")
	}
	if err := h.svc.DisableTOTP(c.Request().Context(), UserID(c), in.Code); err != nil {
		return authErr(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) refresh(c echo.Context) error {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&in); err != nil || in.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refresh_token obrigatório")
	}
	tk, err := h.svc.Refresh(c.Request().Context(), in.RefreshToken, clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"tokens": tk})
}

func (h *Handler) logout(c echo.Context) error {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.Bind(&in)
	if err := h.svc.Logout(c.Request().Context(), in.RefreshToken); err != nil {
		return authErr(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- middleware ---

const ctxUserID = "user_id"
const ctxRole = "role"
const ctxMFA = "mfa"

// privilegedRoles precisam de 2FA para chegar às rotas sensíveis.
var privilegedRoles = map[string]bool{"admin": true, "moderator": true, "organizer": true}

// Middleware exige um Bearer token válido e injeta user_id/role no contexto.
func (h *Handler) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			raw := c.Request().Header.Get(echo.HeaderAuthorization)
			if !strings.HasPrefix(raw, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "token ausente")
			}
			claims, err := h.svc.ParseAccess(strings.TrimPrefix(raw, "Bearer "))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "token inválido")
			}
			c.Set(ctxUserID, claims.Subject)
			c.Set(ctxRole, claims.Role)
			c.Set(ctxMFA, claims.MFA)
			return next(c)
		}
	}
}

// RequireMFA barra a requisição quando a sessão não passou pelo 2FA. Use em
// rotas sensíveis. Contas privilegiadas (admin/moderator/organizer) são sempre
// exigidas; para as demais, passe alwaysForRunner = true.
func RequireMFA(alwaysForRunner bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(ctxRole).(string)
			verified, _ := c.Get(ctxMFA).(bool)
			if verified {
				return next(c)
			}
			if alwaysForRunner || privilegedRoles[role] {
				return echo.NewHTTPError(http.StatusForbidden, "2FA obrigatório para esta ação")
			}
			return next(c)
		}
	}
}

// UserID lê o id do usuário autenticado do contexto.
func UserID(c echo.Context) string {
	v, _ := c.Get(ctxUserID).(string)
	return v
}

// MeHandler é um endpoint protegido de exemplo (GET /v1/me).
func (h *Handler) MeHandler(c echo.Context) error {
	u, err := h.svc.Me(c.Request().Context(), UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "sessão inválida")
	}
	return c.JSON(http.StatusOK, publicUser(u))
}

// --- helpers ---

func publicUser(u User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"athlete_id":     u.AthleteID,
		"username":       u.Username,
		"email":          u.Email,
		"role":           u.Role,
		"email_verified": u.EmailVerified,
	}
}

func clientIP(c echo.Context) string { return c.RealIP() }

func authErr(err error) error {
	switch {
	case errors.Is(err, ErrEmailTaken), errors.Is(err, ErrUsernameTaken):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCreds), errors.Is(err, ErrSessionInvalid),
		errors.Is(err, ErrTokenReused), errors.Is(err, ErrAccountBlocked),
		errors.Is(err, ErrMFARequired), errors.Is(err, ErrMFAInvalidCode):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrMFAAlreadyOn):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ErrMFANotPending), errors.Is(err, ErrMFANotEnabled):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrOAuthProvider):
		return echo.NewHTTPError(http.StatusNotImplemented, err.Error())
	case errors.Is(err, ErrOAuthState), errors.Is(err, ErrOAuthExchange):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrIdentityLinked):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ErrWeakPassword):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrResetInvalid), errors.Is(err, ErrVerifyInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrAlreadyVerified):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
}
