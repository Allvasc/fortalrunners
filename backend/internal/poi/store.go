package poi

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type POI struct {
	ID       string  `json:"id"`
	CityID   string  `json:"city_id"`
	Name     string  `json:"name"`
	Category string  `json:"category"` // bebedouro, banheiro, hidratacao, emergencia, sombra
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Note     string  `json:"note,omitempty"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListPOIs(ctx context.Context, cityID string, category string) ([]POI, error) {
	query := `
		SELECT id, city_id, name, category, ST_Y(geom) as lat, ST_X(geom) as lng, COALESCE(note, '')
		FROM map_pois
		WHERE status = 'active' AND ($1 = '' OR city_id = $1) AND ($2 = '' OR category = $2)
		ORDER BY category ASC, name ASC
	`
	rows, err := s.pool.Query(ctx, query, cityID, category)
	if err != nil {
		return nil, fmt.Errorf("list pois query: %w", err)
	}
	defer rows.Close()

	var pois []POI
	for rows.Next() {
		var p POI
		if err := rows.Scan(&p.ID, &p.CityID, &p.Name, &p.Category, &p.Lat, &p.Lng, &p.Note); err != nil {
			return nil, fmt.Errorf("scan poi: %w", err)
		}
		pois = append(pois, p)
	}
	if pois == nil {
		pois = []POI{}
	}
	return pois, nil
}
