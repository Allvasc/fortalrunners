package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrResetInvalid = errors.New("link de redefinição inválido ou expirado")

const resetTTL = 30 * time.Minute

// Mailer é a porta de envio de e-mail. O default só registra no log; um provedor
// real (Resend/SendGrid/SES) é plugado depois sem tocar no resto.
type Mailer interface {
	SendPasswordReset(ctx context.Context, to, link string) error
}

type logMailer struct{ log *slog.Logger }

func (m logMailer) SendPasswordReset(_ context.Context, to, link string) error {
	m.log.Info("password reset (sem provedor de e-mail configurado)", "to", to, "link", link)
	return nil
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// RequestPasswordReset sempre responde sem erro (não vaza se o e-mail existe).
func (s *Service) RequestPasswordReset(ctx context.Context, email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.store.userByEmail(ctx, email)
	if err != nil || u.Status != "active" {
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return
	}
	token := hex.EncodeToString(raw)
	exp := time.Now().Add(resetTTL)
	if err := s.store.createResetToken(ctx, sha256hex(token), u.ID, exp); err != nil {
		return
	}

	base := s.webBase
	if base == "" {
		base = "https://fortalrunners-web.onrender.com"
	}
	link := base + "/redefinir-senha?token=" + token
	_ = s.mailer.SendPasswordReset(ctx, email, link)
}

// ResetPassword troca a senha se o token é válido; revoga todas as sessões.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	userID, err := s.store.consumeResetToken(ctx, sha256hex(strings.TrimSpace(token)))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrResetInvalid
	}
	if err != nil {
		return err
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.store.setPassword(ctx, userID, hash); err != nil {
		return err
	}
	_ = s.store.revokeAllSessions(ctx, userID)
	return nil
}
