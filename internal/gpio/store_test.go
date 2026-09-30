package gpio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpio.json")
	store := NewStore(path)

	st := stateFile{Pins: map[string]storedPin{
		"4": {
			Current: PinConfig{Mode: ModeOut, Output: OutputDigital, Value: true},
			Default: &PinConfig{Mode: ModeIn},
		},
	}}
	if err := store.Save(st); err != nil {
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
	store := NewStore(filepath.Join(t.TempDir(), "missing.json"))
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Pins) != 0 {
		t.Fatalf("want empty pins")
	}
}

func TestStoreCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpio.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Pins == nil {
		t.Fatal("expected empty map")
	}
}
