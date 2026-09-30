package cluster

import (
	"net"
	"testing"
	"time"

	"armada/internal/uart"
)

func TestBoardMarksTimeoutOffline(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go fakeNodes(b)

	board := NewBoard(uart.NewClient(uart.Wrap(a, a, a.SetReadDeadline, nil, nil), 2), time.Second)
	board.poll()

	nodes := board.Nodes()
	if nodes[0].Online || nodes[0].Error == "" {
		t.Fatalf("node 1: %+v", nodes[0])
	}
	if !nodes[1].Online || nodes[1].Host == nil || nodes[1].Host.Hostname != "n2" {
		t.Fatalf("node 2: %+v", nodes[1])
	}
}

func fakeNodes(conn net.Conn) {
	port := uart.Wrap(conn, conn, conn.SetReadDeadline, nil, nil)
	selected := 0
	for {
		line, err := port.ReadLine(2 * time.Second)
		if err != nil {
			return
		}
		switch {
		case len(line) > 5 && line[:5] == "!SEL ":
			if line[5] == '1' {
				selected = 1
			} else {
				selected = 2
			}
			_, _ = conn.Write([]byte("OK\n"))
		case line == "STATUS" && selected == 1:
			_, _ = conn.Write([]byte("ERR timeout\n"))
		case line == "STATUS":
			_, _ = conn.Write([]byte("{\"hostname\":\"n2\"}\n"))
		default:
			_, _ = conn.Write([]byte("ERR unknown\n"))
		}
	}
}
