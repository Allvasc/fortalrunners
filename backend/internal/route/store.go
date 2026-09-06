package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var ErrRouteNotFound = errors.New("route not found")

type Route struct {
	ID          string    `json:"id"`
	CreatedBy   *string   `json:"created_by,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DistanceM   int       `json:"distance_m"`
	Surface     string    `json:"surface"`
	IsOfficial  bool      `json:"is_official"`
	GeoJSON     string    `json:"geojson"`
	AvgRating   float64   `json:"avg_rating"`
	ReviewCount int       `json:"review_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type Review struct {
	ID        string    `json:"id"`
	RouteID   string    `json:"route_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name,omitempty"`
	Rating    int       `json:"rating"`
	Tags      []string  `json:"tags"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListRoutes(ctx context.Context, limit int) ([]Route, error) {
	query := `
		SELECT
			r.id, r.created_by, r.name, COALESCE(r.description, ''), r.distance_m,
			r.surface, r.is_official, ST_AsGeoJSON(r.geom) as geojson,
			COALESCE(AVG(rr.rating), 0) as avg_rating,
			COUNT(rr.id) as review_count,
			r.created_at
		FROM routes r
		LEFT JOIN route_reviews rr ON rr.route_id = r.id AND rr.status = 'approved'
		GROUP BY r.id, r.created_by, r.name, r.description, r.distance_m, r.surface, r.is_official, r.geom, r.created_at
		ORDER BY r.is_official DESC, r.created_at DESC
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list routes query: %w", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var r Route
		var createdBy *string
		if err := rows.Scan(
			&r.ID, &createdBy, &r.Name, &r.Description, &r.DistanceM,
			&r.Surface, &r.IsOfficial, &r.GeoJSON, &r.AvgRating, &r.ReviewCount, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		r.CreatedBy = createdBy
		routes = append(routes, r)
	}
	return routes, nil
}

func (s *Store) GetRoute(ctx context.Context, routeID string) (*Route, []Review, error) {
	routeQuery := `
		SELECT 
			r.id, r.created_by, r.name, COALESCE(r.description, ''), r.distance_m, 
			r.surface, r.is_official, ST_AsGeoJSON(r.geom) as geojson,
			COALESCE(AVG(rr.rating), 0) as avg_rating,
			COUNT(rr.id) as review_count,
			r.created_at
		FROM routes r
		LEFT JOIN route_reviews rr ON rr.route_id = r.id AND rr.status = 'approved'
		WHERE r.id = $1
		GROUP BY r.id, r.created_by, r.name, r.description, r.distance_m, r.surface, r.is_official, r.geom, r.created_at
	`
	row := s.pool.QueryRow(ctx, routeQuery, routeID)
	var r Route
	var createdBy *string
	if err := row.Scan(
		&r.ID, &createdBy, &r.Name, &r.Description, &r.DistanceM,
		&r.Surface, &r.IsOfficial, &r.GeoJSON, &r.AvgRating, &r.ReviewCount, &r.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrRouteNotFound
		}
		return nil, nil, fmt.Errorf("get route scan: %w", err)
	}
	r.CreatedBy = createdBy

	reviewsQuery := `
		SELECT rr.id, rr.route_id, rr.user_id, COALESCE(u.display_name, u.username), rr.rating, rr.tags_jsonb, COALESCE(rr.body, ''), rr.created_at
		FROM route_reviews rr
		JOIN users u ON u.id = rr.user_id
		WHERE rr.route_id = $1 AND rr.status = 'approved'
		ORDER BY rr.created_at DESC
	`
	rRows, err := s.pool.Query(ctx, reviewsQuery, routeID)
	if err != nil {
		return nil, nil, fmt.Errorf("query route reviews: %w", err)
	}
	defer rRows.Close()

	var reviews []Review
	for rRows.Next() {
		var rev Review
		var tagsJSON []byte
		if err := rRows.Scan(&rev.ID, &rev.RouteID, &rev.UserID, &rev.UserName, &rev.Rating, &tagsJSON, &rev.Body, &rev.CreatedAt); err == nil {
			_ = json.Unmarshal(tagsJSON, &rev.Tags)
			reviews = append(reviews, rev)
		}
	}

	return &r, reviews, nil
}

var ErrBadGeometry = errors.New("geometria da rota inválida")

// CreateRoute grava a rota. A distância é calculada da própria geometria (não é
// confiada ao cliente) e a geometria é validada (LINESTRING, WGS84, dentro de
// limites plausíveis).
func (s *Store) CreateRoute(ctx context.Context, userID, name, description string, surface string, lineWKT string) (*Route, error) {
	up := strings.ToUpper(strings.TrimSpace(lineWKT))
	if !strings.HasPrefix(up, "LINESTRING") || len(lineWKT) > 60_000 {
		return nil, ErrBadGeometry
	}

	routeID := id.New()
	now := time.Now().UTC()

	query := `
		WITH g AS (SELECT ST_SetSRID(ST_GeomFromText($7), 4326) AS geom)
		INSERT INTO routes (id, created_by, name, description, distance_m, surface, is_official, geom, created_at)
		SELECT $1, $2, $3, $4, ST_Length(g.geom::geography)::int, $5, false, g.geom, $6
		FROM g
		WHERE ST_NPoints(g.geom) BETWEEN 2 AND 5000
		  AND ST_Length(g.geom::geography) BETWEEN 100 AND 200000
		RETURNING distance_m, ST_AsGeoJSON(geom)
	`
	var distanceM int
	var geoJSON string
	err := s.pool.QueryRow(ctx, query, routeID, userID, name, description, surface, now, lineWKT).Scan(&distanceM, &geoJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBadGeometry
	}
	if err != nil {
		return nil, fmt.Errorf("create route insert: %w", err)
	}

	return &Route{
		ID: routeID, CreatedBy: &userID, Name: name, Description: description,
		DistanceM: distanceM, Surface: surface, GeoJSON: geoJSON, CreatedAt: now,
	}, nil
}

func (s *Store) AddReview(ctx context.Context, userID, routeID string, rating int, tags []string, body string) (*Review, error) {
	reviewID := id.New()
	now := time.Now().UTC()

	tagsJSON, _ := json.Marshal(tags)

	// entra como 'pending' — fila de moderação do admin (plano §5).
	query := `
		INSERT INTO route_reviews (id, route_id, user_id, rating, tags_jsonb, body, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		ON CONFLICT (route_id, user_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			tags_jsonb = EXCLUDED.tags_jsonb,
			body = EXCLUDED.body,
			status = 'pending',
			created_at = EXCLUDED.created_at
		RETURNING id, created_at
	`
	var retID string
	var retTime time.Time
	err := s.pool.QueryRow(ctx, query, reviewID, routeID, userID, rating, tagsJSON, body, now).Scan(&retID, &retTime)
	if err != nil {
		return nil, fmt.Errorf("add review query: %w", err)
	}

	return &Review{
		ID:        retID,
		RouteID:   routeID,
		UserID:    userID,
		Rating:    rating,
		Tags:      tags,
		Body:      body,
		CreatedAt: retTime,
	}, nil
}

// PendingReview é uma linha da fila de moderação de avaliações.
type PendingReview struct {
	ID        string    `json:"id"`
	RouteID   string    `json:"route_id"`
	RouteName string    `json:"route_name"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Rating    int       `json:"rating"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) PendingReviews(ctx context.Context, limit int) ([]PendingReview, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT rr.id, rr.route_id, r.name, rr.user_id, u.username, rr.rating,
		       COALESCE(rr.body, ''), rr.created_at
		FROM route_reviews rr
		JOIN routes r ON r.id = rr.route_id
		JOIN users u ON u.id = rr.user_id
		WHERE rr.status = 'pending'
		ORDER BY rr.created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingReview
	for rows.Next() {
		var p PendingReview
		if err := rows.Scan(&p.ID, &p.RouteID, &p.RouteName, &p.UserID, &p.Username,
			&p.Rating, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) ModerateReview(ctx context.Context, reviewID, decision string) error {
	newStatus := "hidden"
	if decision == "approve" {
		newStatus = "approved"
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE route_reviews SET status = $2 WHERE id = $1 AND status = 'pending'`,
		reviewID, newStatus)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRouteNotFound
	}
	return nil
}
