// Package qr emite e valida o QR pessoal do atleta (plano §3).
//
// O token rotativo é uma string assinada (HMAC-SHA256) de validade curta (~45 s):
//
//	fr1.<base64url(user_id|exp_unix|nonce)>.<hmac_hex>
//
// Print ou captura de tela não servem depois que expira. A leitura SEMPRE
// resolve para a conta dona do ID e é gravada nela; ações sensíveis ficam
// 'pending' até o dono co-confirmar (handshake).
package qr

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrBadToken  = errors.New("token inválido ou expirado")
	ErrRevoked   = errors.New("token revogado")
	ErrSelfScan  = errors.New("não é possível escanear o próprio QR")
	ErrScanScope = errors.New("kind de leitura inválido")
)

const rotatingTTL = 45 * time.Second

// kinds que exigem co-confirmação do dono antes de valer (plano §3).
var sensitiveKinds = map[string]bool{"finish": true, "lap": true, "checkpoint": true, "sponsor": true}

var validKinds = map[string]bool{
	"kit": true, "start": true, "checkpoint": true, "finish": true,
	"lap": true, "aid": true, "sponsor": true, "friend": true, "landmark": true,
}

type QRToken struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	PayloadSig string    `json:"payload_sig"` // string que vai no QR
	ExpiresAt  time.Time `json:"expires_at"`
}

type ScanResult struct {
	ID        string    `json:"id"`
	AthleteID string    `json:"athlete_id"` // número público do dono (FR-000...)
	Username  string    `json:"username"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"` // recorded | pending
	ScannedAt time.Time `json:"scanned_at"`
}

type Service struct {
	pool   *pgxpool.Pool
	secret []byte
}

func NewService(pool *pgxpool.Pool, signingSecret string) *Service {
	return &Service{pool: pool, secret: []byte(signingSecret)}
}

func (s *Service) sign(msg string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// GetToken emite um token rotativo para o usuário logado.
func (s *Service) GetToken(ctx context.Context, userID, eventID string) (*QRToken, error) {
	nonce := make([]byte, 9)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	exp := time.Now().Add(rotatingTTL)
	body := base64.RawURLEncoding.EncodeToString([]byte(
		userID + "|" + strconv.FormatInt(exp.Unix(), 10) + "|" + base64.RawURLEncoding.EncodeToString(nonce),
	))
	payload := "fr1." + body + "." + s.sign(body)

	tokID := id.New()
	q := `
		INSERT INTO qr_tokens (id, user_id, kind, payload_sig, event_id, expires_at)
		VALUES ($1, $2, 'rotating', $3, NULLIF($4,''), $5)
		RETURNING id, kind, payload_sig, expires_at`
	var tok QRToken
	if err := s.pool.QueryRow(ctx, q, tokID, userID, payload, eventID, exp).Scan(
		&tok.ID, &tok.Kind, &tok.PayloadSig, &tok.ExpiresAt,
	); err != nil {
		return nil, fmt.Errorf("emitir token qr: %w", err)
	}
	return &tok, nil
}

// verify decodifica e valida o payload assinado, devolvendo o user_id do dono.
func (s *Service) verify(payload string) (string, error) {
	parts := strings.Split(payload, ".")
	if len(parts) != 3 || parts[0] != "fr1" {
		return "", ErrBadToken
	}
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(parts[1]))) {
		return "", ErrBadToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrBadToken
	}
	f := strings.Split(string(raw), "|")
	if len(f) != 3 {
		return "", ErrBadToken
	}
	exp, err := strconv.ParseInt(f[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", ErrBadToken
	}
	return f[0], nil
}

// ScanToken registra uma passagem NA CONTA DO DONO do QR.
func (s *Service) ScanToken(ctx context.Context, scannedBy, payload, kind string) (*ScanResult, error) {
	if kind == "" {
		kind = "checkpoint"
	}
	if !validKinds[kind] {
		return nil, ErrScanScope
	}

	ownerID, err := s.verify(payload)
	if err != nil {
		return nil, err
	}
	if ownerID == scannedBy {
		return nil, ErrSelfScan
	}

	// o token precisa existir, não estar revogado e não ter expirado no banco.
	var revoked bool
	var eventID *string
	err = s.pool.QueryRow(ctx,
		`SELECT revoked, event_id FROM qr_tokens WHERE payload_sig = $1 AND user_id = $2 AND expires_at > now()`,
		payload, ownerID).Scan(&revoked, &eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBadToken
	}
	if err != nil {
		return nil, fmt.Errorf("carregar token: %w", err)
	}
	if revoked {
		return nil, ErrRevoked
	}

	status := "recorded"
	if sensitiveKinds[kind] {
		status = "pending" // aguarda co-confirmação do dono
	}

	scanID := id.New()
	var res ScanResult
	q := `
		WITH ins AS (
			INSERT INTO scan_events (id, athlete_id, event_id, scanned_by, kind, status)
			VALUES ($1, $2, $3, $4, $5::scan_kind, $6::scan_status)
			RETURNING id, athlete_id, kind, status, scanned_at
		)
		SELECT ins.id, u.athlete_id, u.username, ins.kind, ins.status, ins.scanned_at
		FROM ins JOIN users u ON u.id = ins.athlete_id`
	if err := s.pool.QueryRow(ctx, q, scanID, ownerID, eventID, scannedBy, kind, status).Scan(
		&res.ID, &res.AthleteID, &res.Username, &res.Kind, &res.Status, &res.ScannedAt,
	); err != nil {
		return nil, fmt.Errorf("gravar leitura: %w", err)
	}
	return &res, nil
}

// ConfirmScan: o dono do ID co-confirma uma leitura sensível pendente.
func (s *Service) ConfirmScan(ctx context.Context, ownerID, scanID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE scan_events SET status = 'recorded'
		 WHERE id = $1 AND athlete_id = $2 AND status = 'pending'`, scanID, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBadToken
	}
	return nil
}

// MyScans devolve o histórico de leituras da conta do usuário (transparência / antifraude).
func (s *Service) MyScans(ctx context.Context, ownerID string, limit int) ([]ScanResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, u.athlete_id, u.username, s.kind, s.status, s.scanned_at
		FROM scan_events s JOIN users u ON u.id = s.athlete_id
		WHERE s.athlete_id = $1
		ORDER BY s.scanned_at DESC LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScanResult
	for rows.Next() {
		var r ScanResult
		if err := rows.Scan(&r.ID, &r.AthleteID, &r.Username, &r.Kind, &r.Status, &r.ScannedAt); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}
