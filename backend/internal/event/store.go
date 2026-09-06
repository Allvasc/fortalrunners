package event

import (
	"context"
	"errors"
	"fmt"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPaidEvent: o evento tem lote pago — a inscrição precisa passar pelo checkout.
var ErrPaidEvent = errors.New("evento pago: use o checkout")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListEvents(ctx context.Context, userID string) ([]Event, error) {
	query := `
		SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.type, e.starts_at, e.ends_at, e.location_name, e.status, e.created_at,
		       EXISTS (SELECT 1 FROM event_participants ep WHERE ep.event_id = e.id AND ep.user_id = $1) as is_registered
		FROM events e
		ORDER BY e.starts_at ASC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.Slug, &ev.OrganizerID, &ev.Title, &ev.Description, &ev.Type, &ev.StartsAt, &ev.EndsAt, &ev.LocationName, &ev.Status, &ev.CreatedAt, &ev.IsRegistered); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

func (s *Store) GetEvent(ctx context.Context, eventID, userID string) (*Event, error) {
	query := `
		SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.type, e.starts_at, e.ends_at, e.location_name, e.status, e.created_at,
		       EXISTS (SELECT 1 FROM event_participants ep WHERE ep.event_id = e.id AND ep.user_id = $2) as is_registered
		FROM events e
		WHERE e.id = $1 OR e.slug = $1
	`
	var ev Event
	if err := s.pool.QueryRow(ctx, query, eventID, userID).Scan(
		&ev.ID, &ev.Slug, &ev.OrganizerID, &ev.Title, &ev.Description, &ev.Type, &ev.StartsAt, &ev.EndsAt, &ev.LocationName, &ev.Status, &ev.CreatedAt, &ev.IsRegistered,
	); err != nil {
		return nil, err
	}

	pQuery := `SELECT id, event_id, name, category, amount_cents, starts_at, ends_at, quota, sold FROM event_prices WHERE event_id = $1`
	pRows, err := s.pool.Query(ctx, pQuery, ev.ID)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p Price
			if err := pRows.Scan(&p.ID, &p.EventID, &p.Name, &p.Category, &p.AmountCents, &p.StartsAt, &p.EndsAt, &p.Quota, &p.Sold); err == nil {
				ev.Prices = append(ev.Prices, p)
			}
		}
	}

	return &ev, nil
}

// RegisterParticipant inscreve o corredor num evento GRATUITO. Eventos com lote
// pago (event_prices.amount_cents > 0) precisam passar por POST /v1/payments/checkout
// — a inscrição só é emitida quando o pagamento confirma (payment.confirmParticipant).
func (s *Store) RegisterParticipant(ctx context.Context, userID, eventID, category, shirtSize string) (*Participant, error) {
	// resolve slug → id e checa se há lote pago, numa query só.
	var realID string
	var hasPaid bool
	err := s.pool.QueryRow(ctx, `
		SELECT e.id, EXISTS(SELECT 1 FROM event_prices p WHERE p.event_id = e.id AND p.amount_cents > 0)
		FROM events e WHERE e.id = $1 OR e.slug = $1`, eventID).Scan(&realID, &hasPaid)
	if err != nil {
		return nil, fmt.Errorf("resolve event: %w", err)
	}
	if hasPaid {
		return nil, ErrPaidEvent
	}

	pid := id.New()
	bib := fmt.Sprintf("FR-%s", pid[len(pid)-6:])
	query := `
		INSERT INTO event_participants (event_id, user_id, bib_number, category, shirt_size)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_id, user_id) DO UPDATE SET category = EXCLUDED.category, shirt_size = EXCLUDED.shirt_size
		RETURNING event_id, user_id, bib_number, category, shirt_size, joined_at
	`
	var p Participant
	if err := s.pool.QueryRow(ctx, query, realID, userID, bib, category, shirtSize).Scan(
		&p.EventID, &p.UserID, &p.BibNumber, &p.Category, &p.ShirtSize, &p.JoinedAt,
	); err != nil {
		return nil, fmt.Errorf("register participant: %w", err)
	}
	return &p, nil
}
