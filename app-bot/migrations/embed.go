// Package migrations embeds the numbered SQL migration set.
//
// @file      migrations/embed.go
// @for       The single source of migration SQL for the binary.
// @uses      embed.
// @reason    A binary that reads migrations from disk can start with a different set than it was built with, so the set ships inside the binary.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package migrations

import "embed"

// FS holds every .up.sql and .down.sql file in this directory.
//
//go:embed *.sql
var FS embed.FS
