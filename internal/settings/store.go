package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	KeySystemName  = "system_name"
	KeyAutoRefresh = "auto_refresh"

	DefaultSystemName = "Armada"
	maxSystemNameLen  = 128
)

// Values is the user-facing settings snapshot for the API.
type Values struct {
	SystemName  string `json:"systemName"`
	AutoRefresh bool   `json:"autoRefresh"`
}

// Store persists settings in SQLite.
type Store struct {
	path string
	db   *sql.DB
}

// Open opens or creates settings.db in dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "settings.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{path: path, db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	return err
}

// Get returns settings with defaults for missing keys.
func (s *Store) Get() (Values, error) {
	out := Values{
		SystemName:  DefaultSystemName,
		AutoRefresh: true,
	}
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return out, err
		}
		switch k {
		case KeySystemName:
			if v != "" {
				out.SystemName = v
			}
		case KeyAutoRefresh:
			out.AutoRefresh = v == "1" || strings.EqualFold(v, "true")
		}
	}
	return out, rows.Err()
}

// Put validates and saves settings.
func (s *Store) Put(v Values) error {
	name := strings.TrimSpace(v.SystemName)
	if name == "" {
		return errors.New("system name required")
	}
	if len(name) > maxSystemNameLen {
		return fmt.Errorf("system name too long (max %d)", maxSystemNameLen)
	}
	auto := "0"
	if v.AutoRefresh {
		auto = "1"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		KeySystemName, name,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		KeyAutoRefresh, auto,
	); err != nil {
		return err
	}
	return tx.Commit()
}
