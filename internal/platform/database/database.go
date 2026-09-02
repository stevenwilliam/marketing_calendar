// Package database opens the pool and runs migrations.
//
// Migrations are the source of truth and are forward-only in production. Each
// applied migration records the SHA-256 of the file it ran, and a drift is
// REFUSED at boot: a migration edited after it was applied means the schema in
// front of you is not the schema the file describes, and every later
// assumption is built on that.
package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Open(dsn string, debug bool) (*gorm.DB, error) {
	lvl := gormlogger.Silent
	if debug {
		lvl = gormlogger.Warn
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(lvl),
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("tidak dapat terhubung ke basis data: %w", err)
	}
	return db, nil
}

type Migration struct {
	Version  int
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string
}

// Load reads NNNN_name.up.sql / .down.sql pairs. A migration WITHOUT a
// matching .down.sql is an error, not a warning: the pair is the contract.
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	byVersion := map[int]*Migration{}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".sql") {
			continue
		}
		isUp := strings.HasSuffix(n, ".up.sql")
		base := strings.TrimSuffix(strings.TrimSuffix(n, ".up.sql"), ".down.sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("nama migrasi tidak sesuai NNNN_name: %s", n)
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("nomor migrasi tidak valid: %s", n)
		}
		body, err := fs.ReadFile(fsys, dir+"/"+n)
		if err != nil {
			return nil, err
		}
		m, ok := byVersion[v]
		if !ok {
			m = &Migration{Version: v, Name: parts[1]}
			byVersion[v] = m
		}
		if isUp {
			m.UpSQL = string(body)
			sum := sha256.Sum256(body)
			m.Checksum = hex.EncodeToString(sum[:])
		} else {
			m.DownSQL = string(body)
		}
	}
	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.UpSQL == "" {
			return nil, fmt.Errorf("migrasi %04d_%s tidak memiliki .up.sql", m.Version, m.Name)
		}
		if m.DownSQL == "" {
			return nil, fmt.Errorf("migrasi %04d_%s tidak memiliki .down.sql — pasangan itu kontraknya", m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

const schemaTable = `
CREATE TABLE IF NOT EXISTS schema_migration (
    version    integer PRIMARY KEY,
    name       text NOT NULL,
    checksum   text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

type Applied struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt time.Time
}

func Status(db *gorm.DB, migrations []Migration) ([]Applied, []Migration, error) {
	if err := db.Exec(schemaTable).Error; err != nil {
		return nil, nil, err
	}
	var applied []Applied
	rows, err := db.Raw(`SELECT version, name, checksum, applied_at FROM schema_migration ORDER BY version`).Rows()
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	seen := map[int]string{}
	for rows.Next() {
		var a Applied
		if err := rows.Scan(&a.Version, &a.Name, &a.Checksum, &a.AppliedAt); err != nil {
			return nil, nil, err
		}
		applied = append(applied, a)
		seen[a.Version] = a.Checksum
	}
	// A checksum that no longer matches means the file was edited after it
	// ran. Refusing here is the whole point: the schema in front of you is not
	// the schema the file describes.
	for _, m := range migrations {
		if sum, ok := seen[m.Version]; ok && sum != m.Checksum {
			return nil, nil, fmt.Errorf(
				"migrasi %04d_%s sudah diterapkan dengan checksum %s tetapi berkasnya sekarang %s — "+
					"berkas migrasi diubah setelah dijalankan; skema tidak lagi sesuai berkasnya",
				m.Version, m.Name, sum[:12], m.Checksum[:12])
		}
	}
	var pending []Migration
	for _, m := range migrations {
		if _, ok := seen[m.Version]; !ok {
			pending = append(pending, m)
		}
	}
	return applied, pending, nil
}

// Up applies pending migrations, each in its own transaction so a failure
// leaves the database at a known version rather than half-migrated.
func Up(db *gorm.DB, migrations []Migration) ([]Migration, error) {
	_, pending, err := Status(db, migrations)
	if err != nil {
		return nil, err
	}
	var done []Migration
	for _, m := range pending {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(m.UpSQL).Error; err != nil {
				return fmt.Errorf("migrasi %04d_%s gagal: %w", m.Version, m.Name, err)
			}
			return tx.Exec(
				`INSERT INTO schema_migration (version, name, checksum) VALUES (?, ?, ?)`,
				m.Version, m.Name, m.Checksum).Error
		})
		if err != nil {
			return done, err
		}
		done = append(done, m)
	}
	return done, nil
}

// Down rolls back the single most recent migration. It exists for development;
// production is forward-only (99 §6).
func Down(db *gorm.DB, migrations []Migration) (*Migration, error) {
	applied, _, err := Status(db, migrations)
	if err != nil {
		return nil, err
	}
	if len(applied) == 0 {
		return nil, nil
	}
	last := applied[len(applied)-1]
	var m *Migration
	for i := range migrations {
		if migrations[i].Version == last.Version {
			m = &migrations[i]
			break
		}
	}
	if m == nil {
		return nil, fmt.Errorf("migrasi %d terpasang tetapi berkasnya tidak ada", last.Version)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(m.DownSQL).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM schema_migration WHERE version = ?`, m.Version).Error
	})
	return m, err
}

// InTx runs fn in a transaction. Every write path that must be atomic uses it.
func InTx(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}

var ErrNoRows = sql.ErrNoRows
