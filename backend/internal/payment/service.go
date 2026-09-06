package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Order struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	AmountCents  int       `json:"amount_cents"`
	AsaasChargeID string   `json:"asaas_charge_id"`
	PixCode      string    `json:"pix_code,omitempty"`
	QrCodeURL    string    `json:"qr_code_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Checkout(ctx context.Context, userID, kind string, amountCents int, method string) (*Order, error) {
	orderID := id.New()
	payID := id.New()
	asaasID := fmt.Sprintf("pay_asaas_%s", orderID[:8])
	pixPayload := fmt.Sprintf("00020126580014BR.GOV.BCB.PIX0136fortalrunners-%s5204000053039865405%d.005802BR5914FortalRunners6009Fortaleza62070503***6304", orderID[:8], amountCents/100)
	qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=250x250&data=%s", pixPayload)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	orderQuery := `
		INSERT INTO orders (id, user_id, kind, status, amount_cents, asaas_customer_id)
		VALUES ($1, $2, $3, 'pending', $4, 'cus_asaas_demo')
		RETURNING id, user_id, kind, status, amount_cents, created_at
	`
	var ord Order
	if err := tx.QueryRow(ctx, orderQuery, orderID, userID, kind, amountCents).Scan(
		&ord.ID, &ord.UserID, &ord.Kind, &ord.Status, &ord.AmountCents, &ord.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	payQuery := `
		INSERT INTO payments (id, order_id, asaas_charge_id, method, status, net_amount_cents, receipt_url)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6)
	`
	if _, err := tx.Exec(ctx, payQuery, payID, orderID, asaasID, method, amountCents, qrURL); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	ord.AsaasChargeID = asaasID
	ord.PixCode = pixPayload
	ord.QrCodeURL = qrURL
	return &ord, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, asaasChargeID, status string) error {
	var ordStatus string
	switch status {
	case "PAYMENT_RECEIVED", "CONFIRMED":
		ordStatus = "paid"
	case "REFUNDED":
		ordStatus = "refunded"
	default:
		ordStatus = "pending"
	}

	query := `
		UPDATE payments SET status = $1, paid_at = now()
		WHERE asaas_charge_id = $2
		RETURNING order_id
	`
	var orderID string
	if err := s.pool.QueryRow(ctx, query, status, asaasChargeID).Scan(&orderID); err != nil {
		return err
	}

	_, _ = s.pool.Exec(ctx, `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, ordStatus, orderID)
	return nil
}
