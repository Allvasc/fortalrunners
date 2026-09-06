package safety

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrSOSNotFound = errors.New("SOS não encontrado")
	ErrBadPIN      = errors.New("PIN incorreto")
)

type Contact struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Relation  string    `json:"relation,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SOSEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Note       string    `json:"note,omitempty"`
	Status     string    `json:"status"`
	ShareToken string    `json:"share_token,omitempty"`
	ShareURL   string    `json:"share_url,omitempty"`
	Notified   int       `json:"notified_contacts"`
	CreatedAt  time.Time `json:"created_at"`
}

// Beacon é a visão PÚBLICA de um SOS ativo — sem PII, sem histórico (plano §6).
type Beacon struct {
	Status    string    `json:"status"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Athlete   string    `json:"athlete"` // primeiro nome / username, só para contexto
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	pool *pgxpool.Pool
	box  crypto.Box
}

func NewStore(pool *pgxpool.Pool, box crypto.Box) *Store {
	return &Store{pool: pool, box: box}
}

func hashPIN(pin string) string {
	if pin == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("frsos:" + pin))
	return hex.EncodeToString(sum[:])
}

func (s *Store) ListContacts(ctx context.Context, userID string) ([]Contact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, phone, phone_enc, COALESCE(relation, ''), created_at
		FROM safety_contacts WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list safety contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var legacyPhone *string
		var phoneEnc []byte
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &legacyPhone, &phoneEnc, &c.Relation, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		if len(phoneEnc) > 0 {
			if p, err := s.box.Open(phoneEnc); err == nil {
				c.Phone = p
			}
		} else if legacyPhone != nil {
			c.Phone = *legacyPhone
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

func (s *Store) AddContact(ctx context.Context, userID, name, phone, relation string) (*Contact, error) {
	enc, err := s.box.Seal(phone)
	if err != nil {
		return nil, fmt.Errorf("cifrar telefone: %w", err)
	}
	cID := id.New()
	now := time.Now().UTC()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO safety_contacts (id, user_id, name, phone_enc, relation, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, cID, userID, name, enc, relation, now); err != nil {
		return nil, fmt.Errorf("add contact: %w", err)
	}
	return &Contact{ID: cID, UserID: userID, Name: name, Phone: phone, Relation: relation, CreatedAt: now}, nil
}

func (s *Store) DeleteContact(ctx context.Context, userID, contactID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM safety_contacts WHERE id = $1 AND user_id = $2`, contactID, userID)
	return err
}

// contactsToNotify devolve (id) dos contatos de emergência do usuário.
func (s *Store) contactsToNotify(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM safety_contacts WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err == nil {
			ids = append(ids, cid)
		}
	}
	return ids, nil
}

func (s *Store) TriggerSOS(ctx context.Context, userID string, lat, lng float64, note, pin, runID string) (*SOSEvent, []string, error) {
	tokRaw := make([]byte, 18)
	if _, err := rand.Read(tokRaw); err != nil {
		return nil, nil, err
	}
	shareToken := hex.EncodeToString(tokRaw)

	sosID := id.New()
	now := time.Now().UTC()
	q := `
		INSERT INTO sos_events (id, user_id, geom, last_lat, last_lng, note, status,
		                        share_token, cancel_pin_hash, run_id, created_at)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $4, $3, $5, 'active',
		        $6, NULLIF($7,''), NULLIF($8,''), $9)`
	if _, err := s.pool.Exec(ctx, q, sosID, userID, lng, lat, note, shareToken, hashPIN(pin), runID, now); err != nil {
		return nil, nil, fmt.Errorf("trigger sos: %w", err)
	}

	contacts, _ := s.contactsToNotify(ctx, userID)
	for _, cid := range contacts {
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO sos_notifications (id, sos_id, contact_id, channel, status)
			VALUES ($1, $2, $3, 'log', 'queued')`, id.New(), sosID, cid)
	}

	return &SOSEvent{
		ID: sosID, UserID: userID, Lat: lat, Lng: lng, Note: note, Status: "active",
		ShareToken: shareToken, Notified: len(contacts), CreatedAt: now,
	}, contacts, nil
}

// CancelSOS encerra o alerta. Exige o PIN quando um foi definido na abertura.
func (s *Store) CancelSOS(ctx context.Context, userID, sosID, pin string) error {
	var pinHash *string
	err := s.pool.QueryRow(ctx,
		`SELECT cancel_pin_hash FROM sos_events WHERE id = $1 AND user_id = $2 AND status = 'active'`,
		sosID, userID).Scan(&pinHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSOSNotFound
	}
	if err != nil {
		return err
	}
	if pinHash != nil && *pinHash != "" && *pinHash != hashPIN(pin) {
		return ErrBadPIN
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE sos_events SET status = 'resolved', resolved_by = $2, resolved_at = now() WHERE id = $1`,
		sosID, userID)
	return err
}

// UpdateBeacon atualiza a última posição de um SOS ativo (chamado pelo app durante a corrida).
func (s *Store) UpdateBeacon(ctx context.Context, userID, sosID string, lat, lng float64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sos_events
		SET geom = ST_SetSRID(ST_Point($3, $4), 4326), last_lat = $4, last_lng = $3
		WHERE id = $1 AND user_id = $2 AND status = 'active'`, sosID, userID, lng, lat)
	return err
}

// GetBeacon é a consulta pública pelo token — mínima, sem PII, sem histórico.
func (s *Store) GetBeacon(ctx context.Context, token string) (*Beacon, error) {
	var b Beacon
	var lat, lng *float64
	err := s.pool.QueryRow(ctx, `
		SELECT e.status, e.last_lat, e.last_lng,
		       COALESCE(NULLIF(split_part(u.display_name, ' ', 1), ''), u.username),
		       e.created_at, GREATEST(e.created_at, COALESCE(e.resolved_at, e.created_at))
		FROM sos_events e JOIN users u ON u.id = e.user_id
		WHERE e.share_token = $1`, token).Scan(&b.Status, &lat, &lng, &b.Athlete, &b.StartedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSOSNotFound
	}
	if err != nil {
		return nil, err
	}
	if lat != nil {
		b.Lat = *lat
	}
	if lng != nil {
		b.Lng = *lng
	}
	return &b, nil
}
