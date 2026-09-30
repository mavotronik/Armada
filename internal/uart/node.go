package uart

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"armada/internal/host"
)

// Runner starts a local power command such as "reboot" or "poweroff".
type Runner func(name string) error

// Serve answers the node line protocol until the port fails.
// snap is sampled on each STATUS. run may be nil, in which case
// REBOOT and SHUTDOWN are refused.
func Serve(port *Port, snap func() host.Snapshot, run Runner) error {
	for {
		line, err := port.ReadLine(time.Minute)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}
		if err := Handle(port, line, snap, run); err != nil {
			log.Printf("node uart: %v", err)
		}
	}
}

// Handle responds to one command line.
func Handle(port *Port, line string, snap func() host.Snapshot, run Runner) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	switch line {
	case "PING":
		return writeLine(port, "PONG")
	case "STATUS":
		if snap == nil {
			return writeLine(port, "ERR status")
		}
		payload, err := json.Marshal(snap())
		if err != nil {
			return writeLine(port, "ERR status")
		}
		_, err = port.Write(append(payload, '\n'))
		return err
	case "REBOOT":
		return ackRun(port, "reboot", run)
	case "SHUTDOWN":
		return ackRun(port, "poweroff", run)
	default:
		return writeLine(port, "ERR unknown")
	}
}

func ackRun(port *Port, name string, run Runner) error {
	if run == nil {
		return writeLine(port, "ERR "+name)
	}
	if err := writeLine(port, "ACK "+name); err != nil {
		return err
	}
	if err := run(name); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func writeLine(port *Port, s string) error {
	_, err := port.Write([]byte(s + "\n"))
	return err
}
