package gpio

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store persists GPIO configuration to SQLite.
type Store struct {
	path string
	db   *sql.DB
}

// NewStore opens gpio.db in dir. Missing or zero-byte file means no persisted pins.
func NewStore(dir string) (*Store, error) {
	path := filepath.Join(dir, "gpio.db")
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{path: path}, nil
		}
		return nil, err
	}
	if info.Size() == 0 {
		return &Store{path: path}, nil
	}
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

func (s *Store) Path() string {
	return s.path
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS pins (
		id TEXT PRIMARY KEY,
		current_json TEXT NOT NULL,
		default_json TEXT
	)`)
	return err
}

func (s *Store) ensureDB() error {
	if s.db != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	s.db = db
	return s.migrate()
}

func (s *Store) Load() (stateFile, error) {
	var st stateFile
	if s.db == nil {
		st.Pins = map[string]storedPin{}
		return st, nil
	}
	rows, err := s.db.Query(`SELECT id, current_json, default_json FROM pins`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	st.Pins = map[string]storedPin{}
	for rows.Next() {
		var id, curJSON string
		var defJSON sql.NullString
		if err := rows.Scan(&id, &curJSON, &defJSON); err != nil {
			return st, err
		}
		var sp storedPin
		if err := json.Unmarshal([]byte(curJSON), &sp.Current); err != nil {
			continue
		}
		if defJSON.Valid && defJSON.String != "" {
			var def PinConfig
			if err := json.Unmarshal([]byte(defJSON.String), &def); err == nil {
				sp.Default = &def
			}
		}
		st.Pins[id] = sp
	}
	return st, rows.Err()
}

func (s *Store) Save(st stateFile) error {
	if st.Pins == nil {
		st.Pins = map[string]storedPin{}
	}
	if err := s.ensureDB(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM pins`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO pins(id, current_json, default_json) VALUES(?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for id, sp := range st.Pins {
		cur, err := json.Marshal(sp.Current)
		if err != nil {
			return err
		}
		var def []byte
		if sp.Default != nil {
			def, err = json.Marshal(sp.Default)
			if err != nil {
				return err
			}
		}
		if _, err := stmt.Exec(id, string(cur), nullString(def)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Reset() error {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func nullString(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
