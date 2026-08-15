//go:build system && !windows

package systemtest

import (
	"os/exec"
	"syscall"
)

// gracefulSignals reports whether this platform can ask the server to shut down
// rather than kill it, which decides if the graceful shutdown test can run.
const gracefulSignals = true

// terminate sends the signal a service manager sends, which the server's start
// command waits for before stopping its apps and closing its modules.
func terminate(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGTERM)
}
