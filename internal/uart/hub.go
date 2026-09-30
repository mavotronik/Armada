package uart

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"armada/internal/host"
)

// Client talks to the Pro Mini hub. One hardware UART on the LuckFox,
// node links switched by !SEL. Methods take the lock for a whole
// transaction so a reboot cannot split a status poll.
type Client struct {
	port  *Port
	nodes int
	mu    sync.Mutex
}

func NewClient(port *Port, nodes int) *Client {
	return &Client{port: port, nodes: nodes}
}

func (c *Client) Nodes() int { return c.nodes }

// Ping checks that the hub firmware is answering.
func (c *Client) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	line, err := c.exchange("!PING", 800*time.Millisecond)
	if err != nil {
		return err
	}
	if line != "OK" {
		return fmt.Errorf("hub: %s", line)
	}
	return nil
}

// Status selects node id (1-based) and reads one metrics snapshot.
func (c *Client) Status(id int) (host.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var zero host.Snapshot
	if err := c.selectLocked(id); err != nil {
		return zero, err
	}
	// Longer than the sketch LINE_MS (4s) so the hub's ERR timeout
	// arrives as a line instead of a local read timeout.
	line, err := c.exchange("STATUS", 5*time.Second)
	if err != nil {
		return zero, err
	}
	if strings.HasPrefix(line, "ERR") || !strings.HasPrefix(line, "{") {
		return zero, fmt.Errorf("%s", line)
	}
	var snap host.Snapshot
	if err := json.Unmarshal([]byte(line), &snap); err != nil {
		return zero, fmt.Errorf("status json: %w", err)
	}
	return snap, nil
}

// Reboot asks the node OS to reboot.
func (c *Client) Reboot(id int) error {
	return c.command(id, "REBOOT", "ACK reboot")
}

// Shutdown asks the node OS to power off.
func (c *Client) Shutdown(id int) error {
	return c.command(id, "SHUTDOWN", "ACK shutdown")
}

// Reset pulses the hub GPIO even if the node OS is dead.
func (c *Client) Reset(id int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.inRange(id); err != nil {
		return err
	}
	line, err := c.exchange(fmt.Sprintf("!RST %d", id), time.Second)
	if err != nil {
		return err
	}
	if line != "OK" {
		return fmt.Errorf("reset: %s", line)
	}
	return nil
}

func (c *Client) command(id int, cmd, expect string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.selectLocked(id); err != nil {
		return err
	}
	line, err := c.exchange(cmd, 3*time.Second)
	if err != nil {
		return err
	}
	if line != expect {
		return fmt.Errorf("%s: %s", strings.ToLower(cmd), line)
	}
	return nil
}

func (c *Client) selectLocked(id int) error {
	if err := c.inRange(id); err != nil {
		return err
	}
	line, err := c.exchange(fmt.Sprintf("!SEL %d", id), 800*time.Millisecond)
	if err != nil {
		return err
	}
	if line != "OK" {
		return fmt.Errorf("sel %d: %s", id, line)
	}
	return nil
}

func (c *Client) inRange(id int) error {
	if id < 1 || id > c.nodes {
		return fmt.Errorf("node %d out of range", id)
	}
	return nil
}

func (c *Client) exchange(cmd string, timeout time.Duration) (string, error) {
	if _, err := c.port.Write([]byte(cmd + "\n")); err != nil {
		return "", err
	}
	line, err := c.port.ReadLine(timeout)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ParseNodeID accepts the {id} path value from the HTTP API.
func ParseNodeID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("bad node id")
	}
	return id, nil
}
