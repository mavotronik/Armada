package cluster

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"armada/internal/host"
	"armada/internal/uart"
)

// Node is the last UART sample for one hub port.
type Node struct {
	ID       int            `json:"id"`
	Online   bool           `json:"online"`
	Error    string         `json:"error,omitempty"`
	LastSeen time.Time      `json:"lastSeen,omitempty"`
	Host     *host.Snapshot `json:"host,omitempty"`
}

// Board polls the UART hub and keeps one slot per node.
type Board struct {
	client   *uart.Client
	interval time.Duration

	mu    sync.RWMutex
	nodes []Node
}

func NewBoard(client *uart.Client, interval time.Duration) *Board {
	n := client.Nodes()
	nodes := make([]Node, n)
	for i := range nodes {
		nodes[i] = Node{ID: i + 1}
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Board{client: client, interval: interval, nodes: nodes}
}

func (b *Board) Nodes() []Node {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Node, len(b.nodes))
	copy(out, b.nodes)
	return out
}

func (b *Board) Run(ctx context.Context) {
	b.poll()
	for {
		timer := time.NewTimer(b.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			b.poll()
		}
	}
}

func (b *Board) poll() {
	for id := 1; id <= len(b.nodes); id++ {
		snap, err := b.client.Status(id)
		b.store(id, snap, err)
	}
}

func (b *Board) store(id int, snap host.Snapshot, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := &b.nodes[id-1]
	was := n.Online
	if err != nil {
		n.Online = false
		if errors.Is(err, uart.ErrTimeout) {
			n.Error = "timeout"
		} else {
			n.Error = err.Error()
		}
	} else {
		n.Online = true
		n.Error = ""
		n.LastSeen = time.Now()
		copied := snap
		n.Host = &copied
	}
	if was != n.Online {
		log.Printf("uart node %d online=%v %s", id, n.Online, n.Error)
	}
}

// Reboot, Shutdown and Reset forward to the hub.
func (b *Board) Reboot(id int) error   { return b.client.Reboot(id) }
func (b *Board) Shutdown(id int) error { return b.client.Shutdown(id) }
func (b *Board) Reset(id int) error    { return b.client.Reset(id) }
