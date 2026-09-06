package club

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var ErrClubNotFound = errors.New("club not found")

type Club struct {
	ID             string    `json:"id"`
	OwnerID        string    `json:"owner_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	NeighborhoodID *string   `json:"neighborhood_id,omitempty"`
	ColorHex       string    `json:"color_hex"`
	MemberCount    int       `json:"member_count"`
	IsMember       bool      `json:"is_member"`
	CreatedAt      time.Time `json:"created_at"`
}

type Member struct {
	UserID    string    `json:"user_id"`
	AthleteID string    `json:"athlete_id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListClubs(ctx context.Context, userID string, limit int) ([]Club, error) {
	query := `
		SELECT
			c.id, c.owner_id, c.name, COALESCE(c.description, ''), c.neighborhood_id, c.color_hex,
			COUNT(cm.user_id) as member_count,
			EXISTS(SELECT 1 FROM club_members cm2 WHERE cm2.club_id = c.id AND cm2.user_id = $1) as is_member,
			c.created_at
		FROM clubs c
		LEFT JOIN club_members cm ON cm.club_id = c.id
		GROUP BY c.id, c.owner_id, c.name, c.description, c.neighborhood_id, c.color_hex, c.created_at
		ORDER BY member_count DESC, c.name ASC
		LIMIT $2
	`
	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list clubs query: %w", err)
	}
	defer rows.Close()

	var clubs []Club
	for rows.Next() {
		var cl Club
		var neighID *string
		if err := rows.Scan(&cl.ID, &cl.OwnerID, &cl.Name, &cl.Description, &neighID, &cl.ColorHex, &cl.MemberCount, &cl.IsMember, &cl.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan club: %w", err)
		}
		cl.NeighborhoodID = neighID
		clubs = append(clubs, cl)
	}
	return clubs, nil
}

func (s *Store) GetClub(ctx context.Context, clubID, userID string) (*Club, []Member, error) {
	clubQuery := `
		SELECT 
			c.id, c.owner_id, c.name, COALESCE(c.description, ''), c.neighborhood_id, c.color_hex,
			COUNT(cm.user_id) as member_count,
			EXISTS(SELECT 1 FROM club_members cm2 WHERE cm2.club_id = c.id AND cm2.user_id = $2) as is_member,
			c.created_at
		FROM clubs c
		LEFT JOIN club_members cm ON cm.club_id = c.id
		WHERE c.id = $1
		GROUP BY c.id, c.owner_id, c.name, c.description, c.neighborhood_id, c.color_hex, c.created_at
	`
	row := s.pool.QueryRow(ctx, clubQuery, clubID, userID)
	var cl Club
	var neighID *string
	if err := row.Scan(&cl.ID, &cl.OwnerID, &cl.Name, &cl.Description, &neighID, &cl.ColorHex, &cl.MemberCount, &cl.IsMember, &cl.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrClubNotFound
		}
		return nil, nil, fmt.Errorf("get club scan: %w", err)
	}
	cl.NeighborhoodID = neighID

	membersQuery := `
		SELECT cm.user_id, u.athlete_id, u.username, COALESCE(u.display_name, u.username), cm.role, cm.joined_at
		FROM club_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.club_id = $1
		ORDER BY cm.joined_at ASC
	`
	mRows, err := s.pool.Query(ctx, membersQuery, clubID)
	if err != nil {
		return nil, nil, fmt.Errorf("query club members: %w", err)
	}
	defer mRows.Close()

	var members []Member
	for mRows.Next() {
		var m Member
		if err := mRows.Scan(&m.UserID, &m.AthleteID, &m.Username, &m.Name, &m.Role, &m.JoinedAt); err == nil {
			members = append(members, m)
		}
	}

	return &cl, members, nil
}

func (s *Store) CreateClub(ctx context.Context, ownerID, name, description string, neighborhoodID *string, colorHex string) (*Club, error) {
	clubID := id.New()
	now := time.Now().UTC()

	if colorHex == "" {
		colorHex = "#0E7C86"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	insClub := `
		INSERT INTO clubs (id, owner_id, name, description, neighborhood_id, color_hex, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := tx.Exec(ctx, insClub, clubID, ownerID, name, description, neighborhoodID, colorHex, now); err != nil {
		return nil, fmt.Errorf("insert club: %w", err)
	}

	insMember := `
		INSERT INTO club_members (club_id, user_id, role, joined_at)
		VALUES ($1, $2, 'owner', $3)
	`
	if _, err := tx.Exec(ctx, insMember, clubID, ownerID, now); err != nil {
		return nil, fmt.Errorf("insert owner member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit club: %w", err)
	}

	return &Club{
		ID:             clubID,
		OwnerID:        ownerID,
		Name:           name,
		Description:    description,
		NeighborhoodID: neighborhoodID,
		ColorHex:       colorHex,
		MemberCount:    1,
		IsMember:       true,
		CreatedAt:      now,
	}, nil
}

func (s *Store) JoinClub(ctx context.Context, userID, clubID string) error {
	query := `
		INSERT INTO club_members (club_id, user_id, role, joined_at)
		VALUES ($1, $2, 'member', now())
		ON CONFLICT (club_id, user_id) DO NOTHING
	`
	_, err := s.pool.Exec(ctx, query, clubID, userID)
	return err
}

// LeaveClub remove o membro. O dono não sai (precisa transferir/apagar antes).
func (s *Store) LeaveClub(ctx context.Context, userID, clubID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM club_members WHERE club_id = $1 AND user_id = $2 AND role <> 'owner'`,
		clubID, userID)
	return err
}

// LeaderEntry: ranking do clube por área de território somada dos membros.
type LeaderEntry struct {
	ClubID      string  `json:"club_id"`
	Name        string  `json:"name"`
	ColorHex    string  `json:"color_hex"`
	MemberCount int     `json:"member_count"`
	AreaM2      float64 `json:"area_m2"`
}

func (s *Store) Leaderboard(ctx context.Context, limit int) ([]LeaderEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.color_hex,
		       COUNT(DISTINCT cm.user_id),
		       COALESCE(SUM(t.area_m2), 0)
		FROM clubs c
		JOIN club_members cm ON cm.club_id = c.id
		LEFT JOIN territories t ON t.user_id = cm.user_id AND t.status = 'active'
		GROUP BY c.id, c.name, c.color_hex
		ORDER BY 5 DESC, 4 DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderEntry
	for rows.Next() {
		var e LeaderEntry
		if err := rows.Scan(&e.ClubID, &e.Name, &e.ColorHex, &e.MemberCount, &e.AreaM2); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
