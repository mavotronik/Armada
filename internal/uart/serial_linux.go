package uart

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Open configures path as an 8N1 raw UART and returns a line port.
// baud is the integer rate (9600, 115200, ...); it must be one this
// process can program with termios.
func Open(path string, baud int) (*Port, error) {
	speed, err := speedFlag(baud)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())
	if err := configure(fd, speed); err != nil {
		f.Close()
		return nil, err
	}
	return Wrap(f, f, f.SetReadDeadline, f.Close, func() error {
		return unix.IoctlSetInt(fd, unix.TCSBRK, 1)
	}), nil
}

func configure(fd int, speed uint32) error {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CSTOPB | unix.CRTSCTS
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL
	t.Cc[unix.VMIN] = 0
	t.Cc[unix.VTIME] = 0
	t.Ispeed = speed
	t.Ospeed = speed
	return unix.IoctlSetTermios(fd, unix.TCSETS, t)
}

func speedFlag(baud int) (uint32, error) {
	switch baud {
	case 9600:
		return unix.B9600, nil
	case 19200:
		return unix.B19200, nil
	case 38400:
		return unix.B38400, nil
	case 57600:
		return unix.B57600, nil
	case 115200:
		return unix.B115200, nil
	default:
		return 0, fmt.Errorf("unsupported baud %d", baud)
	}
}
