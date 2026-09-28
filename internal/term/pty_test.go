//go:build linux

package term

import "testing"

func TestNewShellSession(t *testing.T) {
	s, err := NewShellSession(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Write([]byte("echo ok\n")); err != nil {
		t.Fatal(err)
	}
}
