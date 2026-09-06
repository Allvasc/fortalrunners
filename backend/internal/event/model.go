package event

import "time"

type Event struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	OrganizerID  string    `json:"organizer_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Type         string    `json:"type"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	LocationName string    `json:"location_name"`
	Status       string    `json:"status"`
	Prices       []Price   `json:"prices,omitempty"`
	IsRegistered bool      `json:"is_registered"`
	CreatedAt    time.Time `json:"created_at"`
}

type Price struct {
	ID          string     `json:"id"`
	EventID     string     `json:"event_id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	AmountCents int        `json:"amount_cents"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	Quota       int        `json:"quota"`
	Sold        int        `json:"sold"`
}

type Participant struct {
	EventID    string     `json:"event_id"`
	UserID     string     `json:"user_id"`
	UserHandle string     `json:"user_handle,omitempty"`
	BibNumber  string     `json:"bib_number,omitempty"`
	Category   string     `json:"category"`
	ShirtSize  string     `json:"shirt_size"`
	JoinedAt   time.Time  `json:"joined_at"`
}
