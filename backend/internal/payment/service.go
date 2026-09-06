// Package payment cuida de pedidos, checkout e do webhook do PSP (Asaas).
//
// Regras de segurança (plano §14, §15):
//   - o valor da cobrança é SEMPRE calculado no servidor a partir de event_prices;
//     o cliente nunca informa o preço.
//   - o webhook é público, mas verifica assinatura HMAC e é idempotente por
//     (provider, external_id) via webhook_events.
//   - erros internos não vazam para o cliente.
//
// Não há SDK real do Asaas ainda: sem ASAAS_API_KEY o checkout opera em modo
// sandbox (cobrança "manual", payload PIX claramente marcado como demo). A
// interface do fluxo já está pronta para plugar o PSP real sem reescrever a regra.
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrPriceNotFound  = errors.New("lote de inscrição não encontrado")
	ErrLotClosed      = errors.New("lote fora do período de venda")
	ErrLotSoldOut     = errors.New("lote esgotado")
	ErrAlreadyOrdered = errors.New("você já tem um pedido para este lote")
	ErrBadSignature   = errors.New("assinatura do webhook inválida")
)

// platformFeeCents = R$2,00 fixo + 5% (plano §15, ajustável por organizador depois).
func platformFeeCents(amount int) int { return 200 + amount*5/100 }

type Order struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	AmountCents   int       `json:"amount_cents"`
	EventID       string    `json:"event_id,omitempty"`
	AsaasChargeID string    `json:"asaas_charge_id"`
	Method        string    `json:"method"`
	PixCode       string    `json:"pix_code,omitempty"`
	Sandbox       bool      `json:"sandbox"`
	CreatedAt     time.Time `json:"created_at"`
}

type Config struct {
	AsaasAPIKey        string
	AsaasWebhookSecret string
}

type Service struct {
	pool *pgxpool.Pool
	cfg  Config
}

func NewService(pool *pgxpool.Pool, cfg Config) *Service {
	return &Service{pool: pool, cfg: cfg}
}

func normalizeMethod(m string) string {
	switch strings.ToLower(m) {
	case "pix", "boleto", "card":
		return strings.ToLower(m)
	default:
		return "pix"
	}
}

// Checkout cria um pedido para um lote de inscrição. O valor vem de event_prices,
// nunca do cliente. idemKey (header Idempotency-Key) evita pedidos duplicados.
func (s *Service) Checkout(ctx context.Context, userID, priceID, method, idemKey string) (*Order, error) {
	method = normalizeMethod(method)

	// pedido idempotente: mesma chave → devolve o pedido já criado.
	if idemKey != "" {
		if o, err := s.orderByIdemKey(ctx, userID, idemKey); err == nil {
			return o, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup idempotency: %w", err)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// lote: preço, evento, janela e quota — travando a linha contra corrida.
	var amount, quota, sold int
	var eventID string
	var startsAt, endsAt *time.Time
	priceQ := `
		SELECT amount_cents, event_id, quota, sold, starts_at, ends_at
		FROM event_prices WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, priceQ, priceID).Scan(&amount, &eventID, &quota, &sold, &startsAt, &endsAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPriceNotFound
		}
		return nil, fmt.Errorf("load price: %w", err)
	}
	now := time.Now().UTC()
	if (startsAt != nil && now.Before(*startsAt)) || (endsAt != nil && now.After(*endsAt)) {
		return nil, ErrLotClosed
	}
	if sold >= quota {
		return nil, ErrLotSoldOut
	}

	orderID := id.New()
	payID := id.New()
	fee := platformFeeCents(amount)
	sandbox := s.cfg.AsaasAPIKey == ""
	asaasID := "manual_" + orderID
	pixCode := "DEMO-PIX-" + orderID // sandbox: opaco, não é um BR Code válido

	var ord Order
	ordQ := `
		INSERT INTO orders (id, user_id, kind, status, amount_cents, platform_fee_cents,
		                    event_id, price_id, idempotency_key)
		VALUES ($1, $2, 'event_registration', 'pending', $3, $4, $5, $6, NULLIF($7,''))
		RETURNING id, user_id, kind, status, amount_cents, created_at`
	if err := tx.QueryRow(ctx, ordQ, orderID, userID, amount, fee, eventID, priceID, idemKey).Scan(
		&ord.ID, &ord.UserID, &ord.Kind, &ord.Status, &ord.AmountCents, &ord.CreatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "idempotency_key") {
			return nil, ErrAlreadyOrdered
		}
		return nil, fmt.Errorf("create order: %w", err)
	}

	payQ := `
		INSERT INTO payments (id, order_id, asaas_charge_id, provider, method, status, net_amount_cents)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)`
	provider := "asaas"
	if sandbox {
		provider = "manual"
	}
	if _, err := tx.Exec(ctx, payQ, payID, orderID, asaasID, provider, method, amount-fee); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE event_prices SET sold = sold + 1 WHERE id = $1`, priceID); err != nil {
		return nil, fmt.Errorf("reserve lot: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	ord.EventID = eventID
	ord.AsaasChargeID = asaasID
	ord.Method = method
	ord.PixCode = pixCode
	ord.Sandbox = sandbox
	return &ord, nil
}

func (s *Service) orderByIdemKey(ctx context.Context, userID, idemKey string) (*Order, error) {
	q := `
		SELECT o.id, o.user_id, o.kind, o.status, o.amount_cents, COALESCE(o.event_id,''),
		       COALESCE(p.asaas_charge_id,''), COALESCE(p.method,'pix'), o.created_at
		FROM orders o LEFT JOIN payments p ON p.order_id = o.id
		WHERE o.user_id = $1 AND o.idempotency_key = $2`
	var o Order
	err := s.pool.QueryRow(ctx, q, userID, idemKey).Scan(
		&o.ID, &o.UserID, &o.Kind, &o.Status, &o.AmountCents, &o.EventID,
		&o.AsaasChargeID, &o.Method, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	o.Sandbox = s.cfg.AsaasAPIKey == ""
	return &o, nil
}

// asaasWebhook é o subconjunto do payload que consumimos.
type asaasWebhook struct {
	Event   string `json:"event"`
	Payment struct {
		ID                string `json:"id"`
		Status            string `json:"status"`
		ExternalReference string `json:"externalReference"`
	} `json:"payment"`
}

// HandleWebhook processa um evento do Asaas. Verifica assinatura (quando há
// segredo configurado) e é idempotente por (provider, external_id).
func (s *Service) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	sigOK := false
	if s.cfg.AsaasWebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(s.cfg.AsaasWebhookSecret))
		mac.Write(rawBody)
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(want)) {
			return ErrBadSignature
		}
		sigOK = true
	}

	var wh asaasWebhook
	if err := json.Unmarshal(rawBody, &wh); err != nil {
		return fmt.Errorf("payload inválido: %w", err)
	}
	if wh.Payment.ID == "" {
		return errors.New("payload sem payment.id")
	}

	// idempotência: primeira vez que vemos este external_id?
	evID := id.New()
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_events (id, provider, external_id, event_type, payload_jsonb, signature_ok)
		VALUES ($1, 'asaas', $2, $3, $4, $5)
		ON CONFLICT (provider, external_id) DO NOTHING`,
		evID, wh.Payment.ID, wh.Event, rawBody, sigOK)
	if err != nil {
		return fmt.Errorf("registrar webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil // já processado
	}

	enumStatus, ordStatus := mapAsaasStatus(wh.Payment.Status)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var orderID string
	err = tx.QueryRow(ctx, `
		UPDATE payments SET status = $1, raw_status = $2,
		       paid_at = CASE WHEN $1 IN ('received','confirmed') THEN now() ELSE paid_at END
		WHERE asaas_charge_id = $3
		RETURNING order_id`, enumStatus, wh.Payment.Status, wh.Payment.ID).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		// cobrança desconhecida — registra e sai sem erro (pode ser de outro ambiente)
		_, _ = s.pool.Exec(ctx, `UPDATE webhook_events SET processed_at = now() WHERE id = $1`, evID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("atualizar pagamento: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, ordStatus, orderID); err != nil {
		return fmt.Errorf("atualizar pedido: %w", err)
	}

	if ordStatus == "paid" {
		if err := confirmParticipant(ctx, tx, orderID); err != nil {
			return err
		}
	}
	if ordStatus == "refunded" {
		// devolve a vaga ao lote
		_, _ = tx.Exec(ctx, `
			UPDATE event_prices SET sold = GREATEST(sold - 1, 0)
			WHERE id = (SELECT price_id FROM orders WHERE id = $1)`, orderID)
	}

	if _, err := tx.Exec(ctx, `UPDATE webhook_events SET processed_at = now() WHERE id = $1`, evID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// confirmParticipant emite a inscrição (event_participants) com o pedido pago.
func confirmParticipant(ctx context.Context, tx pgx.Tx, orderID string) error {
	var userID, eventID string
	if err := tx.QueryRow(ctx,
		`SELECT user_id, COALESCE(event_id,'') FROM orders WHERE id = $1`, orderID,
	).Scan(&userID, &eventID); err != nil {
		return fmt.Errorf("carregar pedido: %w", err)
	}
	if eventID == "" {
		return nil // pedido não é de evento
	}
	prefix := eventID
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}
	bib := fmt.Sprintf("%s-%s", strings.ToUpper(prefix), orderID[len(orderID)-4:])
	_, err := tx.Exec(ctx, `
		INSERT INTO event_participants (event_id, user_id, order_id, bib_number)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id, user_id) DO UPDATE SET order_id = EXCLUDED.order_id`,
		eventID, userID, orderID, bib)
	return err
}

// mapAsaasStatus traduz o status do Asaas para o enum pay_status e o status do pedido.
func mapAsaasStatus(s string) (payStatus, orderStatus string) {
	switch strings.ToUpper(s) {
	case "CONFIRMED":
		return "confirmed", "paid"
	case "RECEIVED", "RECEIVED_IN_CASH", "PAYMENT_RECEIVED":
		return "received", "paid"
	case "OVERDUE":
		return "overdue", "pending"
	case "REFUNDED", "REFUND_REQUESTED":
		return "refunded", "refunded"
	case "CHARGEBACK_REQUESTED", "CHARGEBACK_DISPUTE", "AWAITING_CHARGEBACK_REVERSAL":
		return "chargeback", "failed"
	default:
		return "pending", "pending"
	}
}
