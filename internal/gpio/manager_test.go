package gpio

import "testing"

func TestManagerStubDigitalOut(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(store, true)

	if err := m.SetConfig("4", PinConfig{Mode: ModeOut, Output: OutputDigital, Value: true}); err != nil {
		t.Fatal(err)
	}
	snap := m.Snapshot()
	if !snap.Hardware {
		// ok
	}
	var pin *PinRuntime
	for i := range snap.Pins {
		if snap.Pins[i].ID == "4" {
			pin = &snap.Pins[i]
			break
		}
	}
	if pin == nil || pin.Level == nil || !*pin.Level {
		t.Fatalf("expected high level on pin 4: %+v", pin)
	}
}

func TestManagerDefaultSaveReset(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(store, true)

	cfg := PinConfig{Mode: ModeOut, Output: OutputDigital, Value: false}
	if err := m.SetConfig("led", cfg); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveDefault("led"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetConfig("led", PinConfig{Mode: ModeOut, Output: OutputDigital, Value: true}); err != nil {
		t.Fatal(err)
	}
	if err := m.ResetToDefault("led"); err != nil {
		t.Fatal(err)
	}
	snap := m.Snapshot()
	for _, p := range snap.Pins {
		if p.ID == "led" {
			if p.Config.Value {
				t.Fatal("expected default false")
			}
			if p.Default == nil || p.Default.Value {
				t.Fatal("expected saved default false")
			}
			return
		}
	}
	t.Fatal("led not found")
}
