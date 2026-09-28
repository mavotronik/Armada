//go:build linux

package term

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// Session is an interactive shell attached to a PTY.
type Session struct {
	master *os.File
	cmd    *exec.Cmd
	done   chan struct{}
}

func NewShellSession(cols, rows uint16) (*Session, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, err
	}

	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, err
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", n)
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, err
	}

	cmd := exec.Command("/bin/sh", "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	// Setctty+Ctty breaks on some Go versions when the slave is opened in the
	// parent; Setsid with slave stdio is enough for an interactive /bin/sh.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols}); err != nil {
		slave.Close()
		master.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		return nil, err
	}

	_ = slave.Close()

	s := &Session{
		master: master,
		cmd:    cmd,
		done:   make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(s.done)
	}()
	return s, nil
}

func (s *Session) Read(p []byte) (int, error) {
	return s.master.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	return s.master.Write(p)
}

func (s *Session) Resize(cols, rows uint16) error {
	return unix.IoctlSetWinsize(int(s.master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
}

func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.master.Close()
	<-s.done
	return nil
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

// CopyPTY copies from reader to writer until EOF.
func CopyPTY(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
