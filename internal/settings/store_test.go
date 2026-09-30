package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	v, err := s.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v.SystemName != DefaultSystemName || !v.AutoRefresh {
		t.Fatalf("unexpected defaults: %+v", v)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Put(Values{SystemName: "Test Node", AutoRefresh: false}); err != nil {
		t.Fatal(err)
	}
	v, err := s.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v.SystemName != "Test Node" || v.AutoRefresh {
		t.Fatalf("unexpected: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.db")); err != nil {
		t.Fatal(err)
	}
}

func TestPutValidation(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Put(Values{SystemName: "  ", AutoRefresh: true}); err == nil {
		t.Fatal("expected error for empty name")
	}
}
