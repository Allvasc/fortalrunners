package qr

import (
	"context"
	"fmt"
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QRToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Kind       string    `json:"kind"`
	PayloadSig string    `json:"payload_sig"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type ScanResult struct {
	ID        string    `json:"id"`
	AthleteID string    `json:"athlete_id"`
	Kind      string    `json:"kind"`
	ScannedAt time.Time `json:"scanned_at"`
	Status    string    `json:"status"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) GetToken(ctx context.Context, userID, eventID string) (*QRToken, error) {
	tokID := id.New()
	expiresAt := time.Now().Add(45 * time.Second)
	sig := fmt.Sprintf("FORTAL_QR_%s_%d", userID[:8], time.Now().Unix())

	query := `
		INSERT INTO qr_tokens (id, user_id, kind, payload_sig, event_id, expires_at)
		VALUES ($1, $2, 'rotating', $3, NULLIF($4, ''), $5)
		RETURNING id, user_id, kind, payload_sig, expires_at
	`
	var tok QRToken
	err := s.pool.QueryRow(ctx, query, tokID, userID, sig, eventID, expiresAt).Scan(
		&tok.ID, &tok.UserID, &tok.Kind, &tok.PayloadSig, &tok.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("generate qr token: %w", err)
	}
	return &tok, nil
}

func (s *Service) ScanToken(ctx context.Context, scannedBy, tokenSig, kind string) (*ScanResult, error) {
	scanID := id.New()
	if kind == "" {
		kind = "checkpoint"
	}

	query := `
		INSERT INTO scan_events (id, athlete_id, scanned_by, kind, status)
		VALUES ($1, $2, $3, $4, 'recorded')
		RETURNING id, athlete_id, kind, scanned_at, status
	`
	var res ScanResult
	err := s.pool.QueryRow(ctx, query, scanID, scannedBy, scannedBy, kind).Scan(
		&res.ID, &res.AthleteID, &res.Kind, &res.ScannedAt, &res.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("scan qr: %w", err)
	}
	return &res, nil
}
