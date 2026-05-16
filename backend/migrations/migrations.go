// Package migrations exposes the embedded SQL migration files for goose.
package migrations

import "embed"

// FS holds every NNNNNN_*.sql migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
