package migrations

import "embed"

// FS holds ordered SQL migrations that are embedded in the server binary.
//
//go:embed *.sql
var FS embed.FS
