package migrations

import "embed"

// FS contains forward-only application migrations.
//
//go:embed *.sql
var FS embed.FS
