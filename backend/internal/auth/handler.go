package auth

import (
	"errors"
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

// Register monta o grupo /v1/auth.
func (h *Handler) Register(g *echo.Group) {
	a := g.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.POST("/refresh", h.refresh)
	a.POST("/logout", h.logout)
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
	u, tk, err := h.svc.Login(c.Request().Context(), in.Email, in.Password, clientIP(c), c.Request().UserAgent())
	if err != nil {
		return authErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"user": publicUser(u), "tokens": tk})
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
		"id":         u.ID,
		"athlete_id": u.AthleteID,
		"username":   u.Username,
		"email":      u.Email,
		"role":       u.Role,
	}
}

func clientIP(c echo.Context) string { return c.RealIP() }

func authErr(err error) error {
	switch {
	case errors.Is(err, ErrEmailTaken), errors.Is(err, ErrUsernameTaken):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCreds), errors.Is(err, ErrSessionInvalid),
		errors.Is(err, ErrTokenReused), errors.Is(err, ErrAccountBlocked):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrWeakPassword):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
}
