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
	ErrBadCoupon      = errors.New("cupom inválido, esgotado ou expirado")
	ErrOrderNotFound  = errors.New("pedido não encontrado")
	ErrNotRefundable  = errors.New("pedido não é reembolsável neste estado")
)

// platformFeeCents = R$2,00 fixo + 5% (plano §15, ajustável por organizador depois).
func platformFeeCents(amount int) int { return 200 + amount*5/100 }

type Order struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	AmountCents   int       `json:"amount_cents"`
	DiscountCents int       `json:"discount_cents,omitempty"`
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
func (s *Service) Checkout(ctx context.Context, userID, priceID, method, couponCode, idemKey string) (*Order, error) {
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

	// cupom (opcional): resolvido e travado dentro da mesma transação.
	discount := 0
	var couponID *string
	if couponCode != "" {
		cid, disc, err := applyCoupon(ctx, tx, couponCode, eventID, amount)
		if err != nil {
			return nil, err
		}
		couponID, discount = &cid, disc
	}
	payable := amount - discount
	if payable < 0 {
		payable = 0
	}

	orderID := id.New()
	payID := id.New()
	fee := platformFeeCents(payable)
	sandbox := s.cfg.AsaasAPIKey == ""
	asaasID := "manual_" + orderID
	pixCode := "DEMO-PIX-" + orderID // sandbox: opaco, não é um BR Code válido

	var ord Order
	ordQ := `
		INSERT INTO orders (id, user_id, kind, status, amount_cents, discount_cents, platform_fee_cents,
		                    event_id, price_id, coupon_id, idempotency_key)
		VALUES ($1, $2, 'event_registration', 'pending', $3, $4, $5, $6, $7, $8, NULLIF($9,''))
		RETURNING id, user_id, kind, status, amount_cents, created_at`
	if err := tx.QueryRow(ctx, ordQ, orderID, userID, payable, discount, fee, eventID, priceID, couponID, idemKey).Scan(
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
	if _, err := tx.Exec(ctx, payQ, payID, orderID, asaasID, provider, method, payable-fee); err != nil {
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
	ord.DiscountCents = discount
	return &ord, nil
}

// applyCoupon valida e consome um cupom dentro da transação de checkout.
func applyCoupon(ctx context.Context, tx pgx.Tx, code, eventID string, amount int) (string, int, error) {
	var (
		id, scope, discType  string
		value, maxUses, used int
		couponEvent          *string
		validUntil           *time.Time
	)
	err := tx.QueryRow(ctx, `
		SELECT id, scope, event_id, discount_type, value, max_uses, used, valid_until
		FROM coupons WHERE lower(code) = lower($1) FOR UPDATE`, code).
		Scan(&id, &scope, &couponEvent, &discType, &value, &maxUses, &used, &validUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrBadCoupon
	}
	if err != nil {
		return "", 0, err
	}
	if validUntil != nil && time.Now().After(*validUntil) {
		return "", 0, ErrBadCoupon
	}
	if maxUses > 0 && used >= maxUses {
		return "", 0, ErrBadCoupon
	}
	if scope == "event" && couponEvent != nil && *couponEvent != eventID {
		return "", 0, ErrBadCoupon
	}

	disc := value
	if discType == "percent" {
		if value < 1 || value > 100 {
			return "", 0, ErrBadCoupon
		}
		disc = amount * value / 100
	}
	if disc > amount {
		disc = amount
	}
	if _, err := tx.Exec(ctx, `UPDATE coupons SET used = used + 1 WHERE id = $1`, id); err != nil {
		return "", 0, err
	}
	return id, disc, nil
}

// RequestRefund abre um pedido de reembolso para um pedido pago do próprio usuário.
// A aprovação/execução acontece no painel de admin (plano §15).
func (s *Service) RequestRefund(ctx context.Context, userID, orderID, reason string) error {
	var payID string
	var amount int
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, o.amount_cents, o.status
		FROM orders o JOIN payments p ON p.order_id = o.id
		WHERE o.id = $1 AND o.user_id = $2`, orderID, userID).Scan(&payID, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if status != "paid" {
		return ErrNotRefundable
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO refunds (id, payment_id, order_id, amount_cents, reason, status, requested_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), 'requested', $6)`,
		id.New(), payID, orderID, amount, reason, userID)
	return err
}

// --- assinatura premium ---

func (s *Service) Subscription(ctx context.Context, userID string) (map[string]any, error) {
	var plan, status string
	var end *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT plan, status, current_period_end FROM subscriptions WHERE user_id = $1`, userID).
		Scan(&plan, &status, &end)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"active": false}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"active": status == "active", "plan": plan, "status": status, "current_period_end": end}, nil
}

func (s *Service) Subscribe(ctx context.Context, userID, plan string) error {
	if plan != "premium_monthly" && plan != "premium_yearly" {
		plan = "premium_monthly"
	}
	end := time.Now().AddDate(0, 1, 0)
	if plan == "premium_yearly" {
		end = time.Now().AddDate(1, 0, 0)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, plan, status, current_period_end)
		VALUES ($1, $2, 'active', $3)
		ON CONFLICT (user_id) DO UPDATE SET plan = EXCLUDED.plan, status = 'active',
		    current_period_end = EXCLUDED.current_period_end, cancel_at = NULL, updated_at = now()`,
		userID, plan, end)
	return err
}

func (s *Service) CancelSubscription(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE subscriptions SET status = 'canceled', cancel_at = current_period_end, updated_at = now()
		WHERE user_id = $1`, userID)
	return err
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
