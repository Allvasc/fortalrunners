// Package config carrega a configuração do processo a partir de variáveis de
// ambiente. Nenhum segredo tem valor padrão utilizável em produção.
package config

import (
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

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	CORSOrigins []string
}

// Load lê o ambiente. Retorna erro quando falta algo obrigatório.
func Load() (Config, error) {
	c := Config{
		Env:                get("APP_ENV", "dev"),
		HTTPAddr:           get("HTTP_ADDR", ":8080"),
		LogLevel:           get("LOG_LEVEL", "info"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           get("REDIS_URL", "redis://localhost:6379/0"),
		NATSURL:            get("NATS_URL", "nats://localhost:4222"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		CORSOrigins:        splitCSV(get("CORS_ORIGINS", "http://localhost:5173")),
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
