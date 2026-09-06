// Package migrations empacota os arquivos SQL de migração no binário.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
