package safety

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
)

// Notifier entrega o alerta de SOS aos contatos (porta — adaptador SMS/push depois).
// O default (logNotifier) só registra; a wiring já fica correta para plugar Twilio/FCM.
type Notifier interface {
	NotifySOS(ctx context.Context, sos *SOSEvent, contactIDs []string)
}

type logNotifier struct{ log *slog.Logger }

func (n logNotifier) NotifySOS(_ context.Context, sos *SOSEvent, contactIDs []string) {
	n.log.Warn("SOS disparado",
		"sos_id", sos.ID, "user_id", sos.UserID,
		"lat", sos.Lat, "lng", sos.Lng,
		"contatos", len(contactIDs), "share_token", sos.ShareToken)
}

type Service struct {
	store     *Store
	notifier  Notifier
	beaconURL string // base para montar o link público (ex.: https://fortalrunners.com)
}

func NewService(pool *pgxpool.Pool, box crypto.Box, beaconBaseURL string, log *slog.Logger) *Service {
	return &Service{
		store:     NewStore(pool, box),
		notifier:  logNotifier{log: log},
		beaconURL: beaconBaseURL,
	}
}

func (s *Service) ListContacts(ctx context.Context, userID string) ([]Contact, error) {
	return s.store.ListContacts(ctx, userID)
}

func (s *Service) AddContact(ctx context.Context, userID, name, phone, relation string) (*Contact, error) {
	return s.store.AddContact(ctx, userID, name, phone, relation)
}

func (s *Service) DeleteContact(ctx context.Context, userID, contactID string) error {
	return s.store.DeleteContact(ctx, userID, contactID)
}

func (s *Service) TriggerSOS(ctx context.Context, userID string, lat, lng float64, note, pin, runID string) (*SOSEvent, error) {
	sos, contacts, err := s.store.TriggerSOS(ctx, userID, lat, lng, note, pin, runID)
	if err != nil {
		return nil, err
	}
	s.notifier.NotifySOS(ctx, sos, contacts)
	if s.beaconURL != "" {
		sos.ShareURL = s.beaconURL + "/s/" + sos.ShareToken
	}
	return sos, nil
}

func (s *Service) CancelSOS(ctx context.Context, userID, sosID, pin string) error {
	return s.store.CancelSOS(ctx, userID, sosID, pin)
}

func (s *Service) UpdateBeacon(ctx context.Context, userID, sosID string, lat, lng float64) error {
	return s.store.UpdateBeacon(ctx, userID, sosID, lat, lng)
}

func (s *Service) Beacon(ctx context.Context, token string) (*Beacon, error) {
	return s.store.GetBeacon(ctx, token)
}
