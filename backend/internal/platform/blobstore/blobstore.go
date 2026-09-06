// Package blobstore guarda arquivos de mídia (fotos de check-in, avatares).
// A interface deixa o domínio independente do backend; hoje há uma implementação
// em Postgres (sobrevive ao restart do container). Trocar por S3/R2 depois é só
// outro adaptador.
package blobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // registra o decoder PNG
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrNotFound   = errors.New("mídia não encontrada")
	ErrBadImage   = errors.New("arquivo não é uma imagem JPEG ou PNG válida")
	ErrTooLarge   = errors.New("imagem acima do limite (8 MB)")
	maxImageBytes = 8 << 20
)

type Blob struct {
	Key         string
	ContentType string
	Bytes       []byte
}

type Store interface {
	// PutImage valida (magic bytes), reprocessa para JPEG (removendo EXIF) e grava.
	PutImage(ctx context.Context, prefix, ownerID, purpose string, raw []byte) (key string, err error)
	Get(ctx context.Context, key string) (*Blob, error)
	OwnerOf(ctx context.Context, key string) (string, error)
}

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

// sniff confere os magic bytes antes de decodificar.
func sniff(raw []byte) (string, bool) {
	switch {
	case len(raw) > 3 && raw[0] == 0xFF && raw[1] == 0xD8 && raw[2] == 0xFF:
		return "jpeg", true
	case len(raw) > 8 && string(raw[:8]) == "\x89PNG\r\n\x1a\n":
		return "png", true
	default:
		return "", false
	}
}

func (s *PGStore) PutImage(ctx context.Context, prefix, ownerID, purpose string, raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", ErrBadImage
	}
	if len(raw) > maxImageBytes {
		return "", ErrTooLarge
	}
	if _, ok := sniff(raw); !ok {
		return "", ErrBadImage
	}

	// decodifica + re-encoda como JPEG: descarta EXIF (geolocalização, device...)
	// e normaliza o formato servido.
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", ErrBadImage
	}
	b := img.Bounds()
	if b.Dx() < 32 || b.Dy() < 32 || b.Dx() > 8000 || b.Dy() > 8000 {
		return "", ErrBadImage
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 82}); err != nil {
		return "", fmt.Errorf("reencode: %w", err)
	}

	key := strings.Trim(prefix, "/") + "/" + id.New() + ".jpg"
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO media (key, owner_id, content_type, bytes, size_bytes, purpose)
		VALUES ($1, $2, 'image/jpeg', $3, $4, $5)`,
		key, nullStr(ownerID), out.Bytes(), out.Len(), purpose); err != nil {
		return "", fmt.Errorf("gravar mídia: %w", err)
	}
	return key, nil
}

func (s *PGStore) Get(ctx context.Context, key string) (*Blob, error) {
	var b Blob
	b.Key = key
	err := s.pool.QueryRow(ctx, `SELECT content_type, bytes FROM media WHERE key = $1`, key).
		Scan(&b.ContentType, &b.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

func (s *PGStore) OwnerOf(ctx context.Context, key string) (string, error) {
	var owner *string
	err := s.pool.QueryRow(ctx, `SELECT owner_id FROM media WHERE key = $1`, key).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if owner == nil {
		return "", nil
	}
	return *owner, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
