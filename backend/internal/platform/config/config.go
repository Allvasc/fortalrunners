// Package config carrega a configuração do processo a partir de variáveis de
// ambiente. Nenhum segredo tem valor padrão utilizável em produção.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env      string // dev | staging | prod
	HTTPAddr string
	LogLevel string

	DatabaseURL string
	RedisURL    string
	NATSURL     string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	MFAEncKey string // base64 de 32 bytes — cifra o segredo TOTP em repouso

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	AppleClientID    string // Services ID (ex.: com.fortalrunners.app.web)
	AppleTeamID      string
	AppleKeyID       string
	AppleP8Key       string // conteúdo do .p8 (PKCS8 EC) — do cofre
	AppleRedirectURL string

	StravaClientID           string
	StravaClientSecret       string
	StravaRedirectURL        string
	StravaWebhookVerifyToken string

	AsaasAPIKey        string // chave do Asaas (cofre) — vazio = modo sandbox/manual
	AsaasWebhookSecret string // segredo HMAC do webhook do Asaas

	AnthropicAPIKey string // chave da Anthropic (coach IA) — vazio = modo sem IA (fallback determinístico)
	AIModel         string // modelo padrão do coach
	WeatherAPIKey   string // chave do provedor de clima — vazio = leitura estática

	OSRMURL string // base de um servidor OSRM (ex.: https://osrm.exemplo.com) — vazio = sem map-matching

	CORSOrigins []string
}

// Load lê o ambiente. Retorna erro quando falta algo obrigatório.
func Load() (Config, error) {
	c := Config{
		Env:                      get("APP_ENV", "dev"),
		HTTPAddr:                 getHTTPAddr(),
		LogLevel:                 get("LOG_LEVEL", "info"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RedisURL:                 get("REDIS_URL", "redis://localhost:6379/0"),
		NATSURL:                  get("NATS_URL", "nats://localhost:4222"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		MFAEncKey:                os.Getenv("MFA_ENC_KEY"),
		GoogleClientID:           os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:       os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:        os.Getenv("GOOGLE_REDIRECT_URL"),
		AppleClientID:            os.Getenv("APPLE_CLIENT_ID"),
		AppleTeamID:              os.Getenv("APPLE_TEAM_ID"),
		AppleKeyID:               os.Getenv("APPLE_KEY_ID"),
		AppleP8Key:               os.Getenv("APPLE_P8_KEY"),
		AppleRedirectURL:         os.Getenv("APPLE_REDIRECT_URL"),
		StravaClientID:           os.Getenv("STRAVA_CLIENT_ID"),
		StravaClientSecret:       os.Getenv("STRAVA_CLIENT_SECRET"),
		StravaRedirectURL:        os.Getenv("STRAVA_REDIRECT_URL"),
		StravaWebhookVerifyToken: os.Getenv("STRAVA_WEBHOOK_VERIFY_TOKEN"),
		AsaasAPIKey:              os.Getenv("ASAAS_API_KEY"),
		AsaasWebhookSecret:       os.Getenv("ASAAS_WEBHOOK_SECRET"),
		AnthropicAPIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		AIModel:                  get("AI_MODEL", "claude-sonnet-5"),
		WeatherAPIKey:            os.Getenv("WEATHER_API_KEY"),
		OSRMURL:                  os.Getenv("OSRM_URL"),
		CORSOrigins:              splitCSV(get("CORS_ORIGINS", "http://localhost:5173")),
	}

	var err error
	if c.AccessTokenTTL, err = dur("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return c, err
	}
	if c.RefreshTokenTTL, err = dur("REFRESH_TOKEN_TTL", 720*time.Hour); err != nil {
		return c, err
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("config: DATABASE_URL é obrigatório")
	}
	if c.IsProd() && len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("config: JWT_SECRET precisa de pelo menos 32 bytes em produção")
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "dev-only-insecure-secret-do-not-use-in-prod"
	}

	if c.MFAEncKey == "" {
		c.MFAEncKey = base64.StdEncoding.EncodeToString([]byte("dev-only-mfa-enc-key-do-not-ship"))
	}
	if raw, err := base64.StdEncoding.DecodeString(c.MFAEncKey); err != nil || len(raw) != 32 {
		c.MFAEncKey = base64.StdEncoding.EncodeToString([]byte("dev-only-mfa-enc-key-do-not-ship"))
	}
	return c, nil
}

func (c Config) IsProd() bool { return c.Env == "prod" }

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func dur(k string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def, fmt.Errorf("config: %s inválido: %w", k, err)
	}
	return d, nil
}

func getHTTPAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		if !strings.HasPrefix(p, ":") {
			return ":" + p
		}
		return p
	}
	return get("HTTP_ADDR", ":8080")
}
