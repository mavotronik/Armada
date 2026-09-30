package uart

import (
	"io"
	"net"
	"testing"
	"time"

	"armada/internal/host"
)

func TestHandleStatusAndPower(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	node := Wrap(a, a, a.SetReadDeadline, nil, nil)
	peer := Wrap(b, b, b.SetReadDeadline, nil, nil)

	snap := host.Snapshot{Hostname: "n1"}
	snap.CPU.UsagePercent = 12.5
	var ran string

	done := make(chan error, 1)
	go func() {
		done <- Handle(node, "STATUS", func() host.Snapshot { return snap }, nil)
	}()

	line, err := peer.ReadLine(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(line) == 0 || line[0] != '{' || !containsAll(line, `"hostname":"n1"`, `"usagePercent":12.5`) {
		t.Fatalf("status line %q", line)
	}

	go func() {
		done <- Handle(node, "REBOOT", nil, func(name string) error {
			ran = name
			return nil
		})
	}()
	line, err = peer.ReadLine(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if line != "ACK reboot" || ran != "reboot" {
		t.Fatalf("reboot line %q ran %q", line, ran)
	}

	go func() {
		done <- Handle(node, "PING", nil, nil)
	}()
	line, err = peer.ReadLine(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if line != "PONG" {
		t.Fatalf("ping %q", line)
	}
}

func TestClientPollAndReset(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	go fakeHub(b, 2)

	client := NewClient(Wrap(a, a, a.SetReadDeadline, nil, nil), 2)
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
	snap, err := client.Status(2)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Hostname != "n2" {
		t.Fatalf("hostname %q", snap.Hostname)
	}
	if _, err := client.Status(3); err == nil {
		t.Fatal("expected range error")
	}
	if err := client.Reboot(1); err != nil {
		t.Fatal(err)
	}
	if err := client.Reset(2); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(1); err == nil {
		t.Fatal("expected offline node")
	}
}

// fakeHub speaks the Pro Mini sketch protocol and pretends to be the nodes.
func fakeHub(conn net.Conn, nodes int) {
	port := Wrap(conn, conn, conn.SetReadDeadline, nil, nil)
	selected := 0
	for {
		line, err := port.ReadLine(2 * time.Second)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				return
			}
			return
		}
		switch {
		case line == "!PING":
			writeLine(port, "OK")
		case len(line) > 5 && line[:5] == "!SEL ":
			id := atoi(line[5:])
			if id < 1 || id > nodes {
				writeLine(port, "ERR range")
				continue
			}
			selected = id
			writeLine(port, "OK")
		case len(line) > 5 && line[:5] == "!RST ":
			id := atoi(line[5:])
			if id < 1 || id > nodes {
				writeLine(port, "ERR range")
				continue
			}
			writeLine(port, "OK")
		case selected == 0:
			writeLine(port, "ERR noselect")
		case line == "STATUS" && selected == 1:
			writeLine(port, "ERR timeout")
		case line == "STATUS":
			writeLine(port, `{"hostname":"n`+itoa(selected)+`"}`)
		case line == "REBOOT":
			writeLine(port, "ACK reboot")
		default:
			writeLine(port, "ERR unknown")
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !bytesContains(s, p) {
			return false
		}
	}
	return true
}

func bytesContains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 || indexOf(s, part) >= 0)
}

func indexOf(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
