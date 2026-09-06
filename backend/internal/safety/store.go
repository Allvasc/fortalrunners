package safety

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
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
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Note      string    `json:"note,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListContacts(ctx context.Context, userID string) ([]Contact, error) {
	query := `
		SELECT id, user_id, name, phone, COALESCE(relation, ''), created_at
		FROM safety_contacts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list safety contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Phone, &c.Relation, &c.CreatedAt); err == nil {
			contacts = append(contacts, c)
		}
	}
	return contacts, nil
}

func (s *Store) AddContact(ctx context.Context, userID, name, phone, relation string) (*Contact, error) {
	cID := id.New()
	now := time.Now().UTC()

	query := `
		INSERT INTO safety_contacts (id, user_id, name, phone, relation, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.pool.Exec(ctx, query, cID, userID, name, phone, relation, now)
	if err != nil {
		return nil, fmt.Errorf("add contact: %w", err)
	}

	return &Contact{
		ID:        cID,
		UserID:    userID,
		Name:      name,
		Phone:     phone,
		Relation:  relation,
		CreatedAt: now,
	}, nil
}

func (s *Store) DeleteContact(ctx context.Context, userID, contactID string) error {
	query := `DELETE FROM safety_contacts WHERE id = $1 AND user_id = $2`
	_, err := s.pool.Exec(ctx, query, contactID, userID)
	return err
}

func (s *Store) TriggerSOS(ctx context.Context, userID string, lat, lng float64, note string) (*SOSEvent, error) {
	sosID := id.New()
	now := time.Now().UTC()

	query := `
		INSERT INTO sos_events (id, user_id, geom, note, status, created_at)
		VALUES ($1, $2, ST_SetSRID(ST_Point($3, $4), 4326), $5, 'active', $6)
	`
	_, err := s.pool.Exec(ctx, query, sosID, userID, lng, lat, note, now)
	if err != nil {
		return nil, fmt.Errorf("trigger sos: %w", err)
	}

	return &SOSEvent{
		ID:        sosID,
		UserID:    userID,
		Lat:       lat,
		Lng:       lng,
		Note:      note,
		Status:    "active",
		CreatedAt: now,
	}, nil
}
