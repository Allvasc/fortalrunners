// Package id gera identificadores ULID (ordenáveis no tempo, sem enumeração).
package id

import (
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// New devolve um ULID em minúsculas como string de 26 caracteres.
func New() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
