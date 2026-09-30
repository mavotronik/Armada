package gpio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpio.db")
	// Create empty file first via Save
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	st := stateFile{Pins: map[string]storedPin{
		"4": {
			Current: PinConfig{Mode: ModeOut, Output: OutputDigital, Value: true},
			Default: &PinConfig{Mode: ModeIn},
		},
	}}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Pins["4"].Current.Value != true {
		t.Fatalf("unexpected config %+v", loaded.Pins["4"])
	}
	if loaded.Pins["4"].Default == nil || loaded.Pins["4"].Default.Mode != ModeIn {
		t.Fatalf("unexpected default %+v", loaded.Pins["4"].Default)
	}
}

func TestStoreMissingFile(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Pins) != 0 {
		t.Fatalf("want empty pins")
	}
}

func TestStoreZeroByteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpio.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Pins) != 0 {
		t.Fatalf("want empty pins")
	}
}

func TestStoreReset(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := stateFile{Pins: map[string]storedPin{
		"4": {Current: PinConfig{Mode: ModeIn}},
	}}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gpio.db")); !os.IsNotExist(err) {
		t.Fatalf("expected gpio.db removed: %v", err)
	}
}
