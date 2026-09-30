package uart

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"
)

// ErrTimeout is a read that produced no full line before the deadline.
var ErrTimeout = errors.New("uart timeout")

// ErrTooLong is a line that exceeded the receive buffer.
var ErrTooLong = errors.New("uart line too long")

const maxLine = 8192

// Port is one UART conversation. Reads and writes must not overlap;
// callers serialize access.
type Port struct {
	r           io.Reader
	w           io.Writer
	setDeadline func(time.Time) error
	closeFn     func() error
	drain       func() error
	buf         []byte
}

// Wrap builds a Port over a byte stream that supports read deadlines.
func Wrap(r io.Reader, w io.Writer, setDeadline func(time.Time) error, closeFn func() error, drain func() error) *Port {
	return &Port{
		r:           r,
		w:           w,
		setDeadline: setDeadline,
		closeFn:     closeFn,
		drain:       drain,
	}
}

func (p *Port) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if err != nil {
		return n, err
	}
	if p.drain != nil {
		if derr := p.drain(); derr != nil {
			return n, derr
		}
	}
	return n, nil
}

func (p *Port) Close() error {
	if p.closeFn == nil {
		return nil
	}
	return p.closeFn()
}

// ReadLine returns the next '\n'-terminated line without the newline.
// A trailing '\r' is removed. timeout bounds the whole read.
func (p *Port) ReadLine(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	tmp := make([]byte, 128)
	for {
		if i := bytes.IndexByte(p.buf, '\n'); i >= 0 {
			line := string(bytes.TrimRight(p.buf[:i], "\r"))
			rest := p.buf[i+1:]
			p.buf = append([]byte(nil), rest...)
			return line, nil
		}
		if len(p.buf) > maxLine {
			p.buf = nil
			return "", ErrTooLong
		}
		if p.setDeadline != nil {
			if err := p.setDeadline(deadline); err != nil {
				return "", err
			}
		}
		n, err := p.r.Read(tmp)
		if n > 0 {
			p.buf = append(p.buf, tmp[:n]...)
		}
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return "", ErrTimeout
			}
			return "", err
		}
		if n == 0 {
			return "", ErrTimeout
		}
	}
}
