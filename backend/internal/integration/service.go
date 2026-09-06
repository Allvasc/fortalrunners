package integration

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
)

var (
	ErrProviderOff  = errors.New("integração não configurada")
	ErrNotConnected = errors.New("conta não conectada")
	ErrState        = errors.New("state inválido")
)

// RunIngester é o que o importador chama (implementado por run.Service).
type RunIngester interface {
	Ingest(ctx context.Context, userID string, in run.IngestInput) (run.View, error)
}

type Config struct {
	JWTSecret string
	Strava    StravaConfig
}

type Service struct {
	pool   *pgxpool.Pool
	box    crypto.Box
	cfg    Config
	strava *stravaClient
	runs   RunIngester
	log    *slog.Logger
}

func NewService(pool *pgxpool.Pool, box crypto.Box, cfg Config, runs RunIngester, log *slog.Logger) *Service {
	hc := &http.Client{Timeout: 20 * time.Second}
	return &Service{
		pool: pool, box: box, cfg: cfg, runs: runs,
		strava: newStrava(cfg.Strava, hc),
		log:    log.With("svc", "integration"),
	}
}

func (s *Service) enabled(provider string) bool {
	return provider == "strava" && s.cfg.Strava.enabled()
}

// --- state assinado ---

func (s *Service) signState(userID string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    "fortalrunners-integ",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
	}).SignedString([]byte(s.cfg.JWTSecret))
}

func (s *Service) parseState(raw string) (string, error) {
	var c jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("fortalrunners-integ"))
	if err != nil || c.Subject == "" {
		return "", ErrState
	}
	return c.Subject, nil
}

// --- fluxo de conexão ---

func (s *Service) ConnectURL(provider, userID string) (string, error) {
	if !s.enabled(provider) {
		return "", ErrProviderOff
	}
	st, err := s.signState(userID)
	if err != nil {
		return "", err
	}
	return s.strava.authURL(st), nil
}

func (s *Service) Complete(ctx context.Context, provider, code, state string) (string, error) {
	if !s.enabled(provider) {
		return "", ErrProviderOff
	}
	userID, err := s.parseState(state)
	if err != nil {
		return "", err
	}
	tok, err := s.strava.exchange(ctx, code)
	if err != nil {
		return "", err
	}
	intID, err := s.upsertIntegration(ctx, userID, provider, tok)
	if err != nil {
		return "", err
	}
	_ = s.enqueueJob(ctx, userID, intID, "backfill")
	return userID, nil
}

func (s *Service) Disconnect(ctx context.Context, provider, userID string, purgeRuns bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE integrations SET status = 'revoked', access_token_enc = NULL, refresh_token_enc = NULL, updated_at = now()
		WHERE user_id = $1 AND provider = $2::integ_provider`, userID, provider)
	if err != nil {
		return err
	}
	if purgeRuns {
		_, err = s.pool.Exec(ctx, `
			UPDATE runs SET status = 'rejected'
			WHERE user_id = $1 AND import_ref LIKE $2`, userID, provider+":%")
	}
	return err
}

func (s *Service) Sync(ctx context.Context, provider, userID string) error {
	var intID string
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM integrations WHERE user_id = $1 AND provider = $2::integ_provider AND status = 'active'`,
		userID, provider).Scan(&intID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotConnected
	}
	if err != nil {
		return err
	}
	return s.enqueueJob(ctx, userID, intID, "delta")
}

// List devolve as conexões do corredor (sem tokens).
func (s *Service) List(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider::text, external_athlete_id, scopes, last_sync_at, status::text, created_at
		FROM integrations WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var prov, ext, st string
		var scopes []string
		var last *time.Time
		var created time.Time
		if err := rows.Scan(&prov, &ext, &scopes, &last, &st, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"provider": prov, "external_athlete_id": ext, "scopes": scopes,
			"last_sync_at": last, "status": st, "connected_at": created,
		})
	}
	return out, rows.Err()
}

// --- webhook ---

// HandleWebhook grava o evento (idempotente) e enfileira um delta sync.
func (s *Service) HandleWebhook(ctx context.Context, provider string, raw []byte) error {
	var ev struct {
		OwnerID    int64  `json:"owner_id"`
		ObjectID   int64  `json:"object_id"`
		AspectType string `json:"aspect_type"`
		ObjectType string `json:"object_type"`
	}
	_ = json.Unmarshal(raw, &ev)
	extID := provider + ":" + strconv.FormatInt(ev.ObjectID, 10) + ":" + ev.AspectType

	tag, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_events (id, provider, external_id, event_type, payload_jsonb, signature_ok, processed_at)
		VALUES ($1, $2, $3, $4, $5, true, now())
		ON CONFLICT (provider, external_id) DO NOTHING`,
		id.New(), provider, extID, ev.ObjectType+"."+ev.AspectType, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // já processado
	}
	if ev.ObjectType != "activity" || ev.AspectType == "delete" {
		return nil
	}
	// acha o dono e enfileira delta
	var userID, intID string
	err = s.pool.QueryRow(ctx, `
		SELECT user_id, id FROM integrations
		WHERE provider = $1::integ_provider AND external_athlete_id = $2 AND status = 'active'`,
		provider, strconv.FormatInt(ev.OwnerID, 10)).Scan(&userID, &intID)
	if err != nil {
		return nil //nolint:nilerr // dono desconhecido — ignora
	}
	return s.enqueueJob(ctx, userID, intID, "delta")
}

func (s *Service) VerifyWebhook(mode, token, challenge string) (string, bool) {
	if mode == "subscribe" && token == s.cfg.Strava.WebhookVerifyToken && challenge != "" {
		return challenge, true
	}
	return "", false
}
