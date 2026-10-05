// Package migrations embeds versioned SQL for the deployment CLI and tests.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

// Source exposes read-only embedded migration files.
func Source() fs.FS { return files }
