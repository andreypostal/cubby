//go:build system && windows

package systemtest

import "os/exec"

// Windows has no signal to deliver to another process: os.Process.Signal rejects
// anything but Kill for a process it did not create a console for. The graceful
// shutdown path is therefore only exercised on the platforms that can ask for it,
// which are also the ones the server is deployed on.
const gracefulSignals = false

func terminate(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
