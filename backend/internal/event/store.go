package event

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	// ErrPaidEvent: o evento tem lote pago — a inscrição precisa passar pelo checkout.
	ErrPaidEvent       = errors.New("evento pago: use o checkout")
	ErrStationNotFound = errors.New("estação do evento não encontrada")
	ErrTooFar          = errors.New("você está longe demais da estação")
	ErrNotRegistered   = errors.New("você não está inscrito neste evento")
	ErrEventNotFound   = errors.New("evento não encontrado")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListEvents(ctx context.Context, userID string, limit int) ([]Event, error) {
	query := `
		SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.type, e.starts_at, e.ends_at, e.location_name, e.status, e.created_at,
		       EXISTS (SELECT 1 FROM event_participants ep WHERE ep.event_id = e.id AND ep.user_id = $1) as is_registered
		FROM events e
		WHERE e.status IN ('live', 'published', 'open') AND e.ends_at > now() - interval '7 days'
		ORDER BY e.starts_at ASC
		LIMIT $2
	`
	rows, err := s.pool.Query(ctx, query, userID, limit)
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

// LeaderEntry: linha do ranking de um evento.
type LeaderEntry struct {
	Rank        int     `json:"rank"`
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	BibNumber   string  `json:"bib_number,omitempty"`
	Checkpoints int     `json:"checkpoints"`
	AreaM2      float64 `json:"area_m2"` // território conquistado durante a janela do evento
	Completed   bool    `json:"completed"`
}

// Leaderboard: participantes ordenados por checkpoints batidos e área conquistada
// durante a janela do evento (plano §3 — "território conquistado durante a prova").
func (s *Store) Leaderboard(ctx context.Context, eventRef string, limit int) ([]LeaderEntry, error) {
	rows, err := s.pool.Query(ctx, `
		WITH e AS (SELECT id, starts_at, ends_at FROM events WHERE id = $1 OR slug = $1)
		SELECT ep.user_id, u.username, COALESCE(ep.bib_number,''),
		       (SELECT count(*) FROM scan_events se
		         WHERE se.event_id = e.id AND se.athlete_id = ep.user_id
		           AND se.kind IN ('checkpoint','start','finish','lap') AND se.status = 'recorded'),
		       COALESCE((SELECT SUM(t.area_m2) FROM territories t, e
		         WHERE t.user_id = ep.user_id AND t.status = 'active'
		           AND t.claimed_at BETWEEN e.starts_at AND e.ends_at), 0),
		       ep.completed_at IS NOT NULL
		FROM event_participants ep
		JOIN e ON ep.event_id = e.id
		JOIN users u ON u.id = ep.user_id
		ORDER BY 4 DESC, 5 DESC
		LIMIT $2`, eventRef, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderEntry
	rank := 0
	for rows.Next() {
		rank++
		var e LeaderEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.BibNumber, &e.Checkpoints, &e.AreaM2, &e.Completed); err != nil {
			return nil, err
		}
		e.Rank = rank
		out = append(out, e)
	}
	return out, nil
}

// CheckpointCheckin registra passagem do participante numa estação do evento
// por proximidade GPS (event_stations com raio).
func (s *Store) CheckpointCheckin(ctx context.Context, userID, eventRef, stationID string, lat, lng float64) (string, error) {
	var eventID, kind string
	var within bool
	err := s.pool.QueryRow(ctx, `
		SELECT e.id, st.role::text,
		       ST_DWithin(st.geom::geography, ST_SetSRID(ST_Point($3,$4),4326)::geography, st.radius_m)
		FROM events e
		JOIN event_stations st ON st.event_id = e.id AND st.id = $2
		WHERE e.id = $1 OR e.slug = $1`, eventRef, stationID, lng, lat).Scan(&eventID, &kind, &within)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrStationNotFound
	}
	if err != nil {
		return "", err
	}
	if !within {
		return "", ErrTooFar
	}

	var registered bool
	_ = s.pool.QueryRow(ctx,
		`SELECT true FROM event_participants WHERE event_id = $1 AND user_id = $2`, eventID, userID).Scan(&registered)
	if !registered {
		return "", ErrNotRegistered
	}

	scanID := id.New()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO scan_events (id, athlete_id, station_id, event_id, scanned_by, kind, geom, status)
		VALUES ($1, $2, $3, $4, $2, $5::scan_kind, ST_SetSRID(ST_Point($6,$7),4326), 'recorded')`,
		scanID, userID, stationID, eventID, kind, lng, lat); err != nil {
		return "", err
	}
	if kind == "finish" {
		_, _ = s.pool.Exec(ctx,
			`UPDATE event_participants SET completed_at = now() WHERE event_id = $1 AND user_id = $2 AND completed_at IS NULL`,
			eventID, userID)
	}
	return kind, nil
}
