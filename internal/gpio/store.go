package gpio

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

// Store persists GPIO configuration to disk.
type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (stateFile, error) {
	var st stateFile
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stateFile{Pins: map[string]storedPin{}}, nil
		}
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		log.Printf("gpio: corrupt state %s: %v", s.path, err)
		return stateFile{Pins: map[string]storedPin{}}, nil
	}
	if st.Pins == nil {
		st.Pins = map[string]storedPin{}
	}
	return st, nil
}

func (s *Store) Save(st stateFile) error {
	if st.Pins == nil {
		st.Pins = map[string]storedPin{}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
