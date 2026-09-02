// Package db embeds the migrations. They are the source of truth (99 §6);
// the ORM models map onto them, never the other way round.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
