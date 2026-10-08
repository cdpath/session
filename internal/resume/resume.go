// Package resume replaces the current process with an agent's resume command.
package resume

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/cdpath/session/internal/agent"
)

// Exec changes into cmd.Dir and replaces this process with the agent. It only
// returns on failure.
func Exec(cmd agent.Command, extra []string) error {
	bin, err := exec.LookPath(cmd.Bin)
	if err != nil {
		return fmt.Errorf("%s is not installed: %w", cmd.Bin, err)
	}
	if err := os.Chdir(cmd.Dir); err != nil {
		return fmt.Errorf("cannot enter %s: %w", cmd.Dir, err)
	}
	argv := append([]string{cmd.Bin}, cmd.Args...)
	argv = append(argv, extra...)
	return syscall.Exec(bin, argv, withPWD(os.Environ(), cmd.Dir))
}

// withPWD keeps $PWD consistent with the new working directory; agents and
// shells they spawn read it.
func withPWD(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "PWD=") {
			out = append(out, kv)
		}
	}
	return append(out, "PWD="+dir)
}

// String renders the command for display, e.g. in notices.
func String(cmd agent.Command, extra []string) string {
	parts := append([]string{cmd.Bin}, cmd.Args...)
	parts = append(parts, extra...)
	for i, p := range parts {
		if strings.ContainsAny(p, " \t'\"") {
			parts[i] = "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
		}
	}
	return strings.Join(parts, " ")
}
