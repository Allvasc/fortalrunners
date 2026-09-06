package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrVerifyInvalid   = errors.New("link de confirmação inválido ou expirado")
	ErrAlreadyVerified = errors.New("e-mail já confirmado")
)

const emailVerifyTTL = 48 * time.Hour

// SendEmailVerification cria um token e dispara o e-mail de confirmação.
// Silencioso se a conta já está verificada (evita e-mail redundante).
func (s *Service) SendEmailVerification(ctx context.Context, userID string) error {
	at, err := s.store.emailVerifiedAt(ctx, userID)
	if err != nil {
		return err
	}
	if at != nil {
		return ErrAlreadyVerified
	}
	u, err := s.store.userByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Email == "" {
		return nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	if err := s.store.createEmailVerifyToken(ctx, sha256hex(token), u.ID, u.Email, time.Now().Add(emailVerifyTTL)); err != nil {
		return err
	}

	base := s.webBase
	if base == "" {
		base = "https://fortalrunners-web.onrender.com"
	}
	return s.mailer.SendEmailVerification(ctx, u.Email, base+"/confirmar-email?token="+token)
}

// VerifyEmail consome o token e carimba users.email_verified_at.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return ErrVerifyInvalid
	}
	_, err := s.store.consumeEmailVerifyToken(ctx, sha256hex(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVerifyInvalid
	}
	return err
}
