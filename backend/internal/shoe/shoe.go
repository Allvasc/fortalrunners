// Package shoe cuida do cadastro de tênis e da quilometragem por par.
package shoe

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrNotFound = errors.New("tênis não encontrado")
	ErrInvalid  = errors.New("marca e modelo são obrigatórios")
)

type Shoe struct {
	ID             string     `json:"id"`
	Brand          string     `json:"brand"`
	Model          string     `json:"model"`
	Nickname       *string    `json:"nickname,omitempty"`
	PurchasedAt    *time.Time `json:"purchased_at,omitempty"`
	PurchasePriceC *int       `json:"purchase_price_cents,omitempty"`
	InitialDistM   int        `json:"initial_distance_m"`
	LifespanGoalM  int        `json:"lifespan_goal_m"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	// derivados
	TotalDistanceM int     `json:"total_distance_m"`
	RunCount       int     `json:"run_count"`
	TotalMovingS   int     `json:"total_moving_s"`
	LifePct        float64 `json:"life_pct"`
	CostPerKmCents *int    `json:"cost_per_km_cents,omitempty"`
	Alert          string  `json:"alert,omitempty"` // "" | trocar_em_breve | vencido
}

type CreateInput struct {
	Brand         string  `json:"brand"`
	Model         string  `json:"model"`
	Nickname      string  `json:"nickname"`
	PurchasedAt   *string `json:"purchased_at"`
	PriceCents    *int    `json:"purchase_price_cents"`
	InitialDistM  int     `json:"initial_distance_m"`
	LifespanGoalM int     `json:"lifespan_goal_m"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Create(ctx context.Context, userID string, in CreateInput) (Shoe, error) {
	in.Brand, in.Model = strings.TrimSpace(in.Brand), strings.TrimSpace(in.Model)
	if in.Brand == "" || in.Model == "" {
		return Shoe{}, ErrInvalid
	}
	if in.LifespanGoalM <= 0 {
		in.LifespanGoalM = 700_000
	}
	sid := id.New()
	var purchased *time.Time
	if in.PurchasedAt != nil && *in.PurchasedAt != "" {
		if d, err := time.Parse("2006-01-02", *in.PurchasedAt); err == nil {
			purchased = &d
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Shoe{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO shoes (id, user_id, brand, model, nickname, purchased_at,
		                   purchase_price_cents, initial_distance_m, lifespan_goal_m)
		VALUES ($1,$2,$3,$4,nullif($5,''),$6,$7,$8,$9)`,
		sid, userID, in.Brand, in.Model, in.Nickname, purchased, in.PriceCents,
		in.InitialDistM, in.LifespanGoalM)
	if err != nil {
		return Shoe{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO shoe_stats (shoe_id) VALUES ($1)`, sid); err != nil {
		return Shoe{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Shoe{}, err
	}
	return s.Get(ctx, userID, sid)
}

func (s *Service) List(ctx context.Context, userID string, includeRetired bool) ([]Shoe, error) {
	q := `
		SELECT s.id, s.brand, s.model, s.nickname, s.purchased_at, s.purchase_price_cents,
		       s.initial_distance_m, s.lifespan_goal_m, s.status::text, s.created_at,
		       COALESCE(st.total_distance_m,0), COALESCE(st.run_count,0), COALESCE(st.total_moving_s,0)
		FROM shoes s LEFT JOIN shoe_stats st ON st.shoe_id = s.id
		WHERE s.user_id = $1`
	if !includeRetired {
		q += ` AND s.status <> 'retired'`
	}
	q += ` ORDER BY s.status, s.created_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Shoe{}
	for rows.Next() {
		sh, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID, shoeID string) (Shoe, error) {
	const q = `
		SELECT s.id, s.brand, s.model, s.nickname, s.purchased_at, s.purchase_price_cents,
		       s.initial_distance_m, s.lifespan_goal_m, s.status::text, s.created_at,
		       COALESCE(st.total_distance_m,0), COALESCE(st.run_count,0), COALESCE(st.total_moving_s,0)
		FROM shoes s LEFT JOIN shoe_stats st ON st.shoe_id = s.id
		WHERE s.id = $1 AND s.user_id = $2`
	sh, err := scan(s.pool.QueryRow(ctx, q, shoeID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sh, ErrNotFound
	}
	return sh, err
}

func (s *Service) Retire(ctx context.Context, userID, shoeID string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE shoes SET status = 'retired', retired_at = now(), updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status <> 'retired'`, shoeID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// OwnedBy confirma que o par pertence ao usuário (usado na ingestão de corrida).
func (s *Service) OwnedBy(ctx context.Context, userID, shoeID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM shoes WHERE id = $1 AND user_id = $2)`, shoeID, userID).Scan(&ok)
	return ok, err
}

func scan(row pgx.Row) (Shoe, error) {
	var sh Shoe
	var totalDist, moving int64
	err := row.Scan(&sh.ID, &sh.Brand, &sh.Model, &sh.Nickname, &sh.PurchasedAt, &sh.PurchasePriceC,
		&sh.InitialDistM, &sh.LifespanGoalM, &sh.Status, &sh.CreatedAt, &totalDist, &sh.RunCount, &moving)
	if err != nil {
		return sh, err
	}
	sh.TotalDistanceM = sh.InitialDistM + int(totalDist)
	sh.TotalMovingS = int(moving)
	derive(&sh)
	return sh, nil
}

// derive calcula os campos apresentados (vida útil %, alerta, custo por km).
func derive(sh *Shoe) {
	if sh.LifespanGoalM > 0 {
		sh.LifePct = float64(sh.TotalDistanceM) / float64(sh.LifespanGoalM) * 100
	}
	switch {
	case sh.LifePct >= 100:
		sh.Alert = "vencido"
	case sh.LifePct >= 75:
		sh.Alert = "trocar_em_breve"
	default:
		sh.Alert = ""
	}
	if sh.PurchasePriceC != nil && sh.TotalDistanceM > 0 {
		c := *sh.PurchasePriceC * 1000 / sh.TotalDistanceM
		sh.CostPerKmCents = &c
	}
}
