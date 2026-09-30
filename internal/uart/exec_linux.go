package uart

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// Exec looks up name on PATH, flushes filesystems, and starts it.
// Start is used so a reboot command is not waited on.
func Exec(name string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	unix.Sync()
	return exec.Command(path).Start()
}
