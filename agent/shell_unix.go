//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// executeShellCommand for Linux and MacOS (Standard Mode)
func executeShellCommand(cmd string) string {
	var command *exec.Cmd
	var out bytes.Buffer

	// Use standard shell /bin/sh
	command = exec.Command("/bin/sh", "-c", cmd)

	// On Linux/Mac, we don't need syscall.HideWindow
	// because agents are usually run as background processes (&)

	command.Stdout = &out
	command.Stderr = &out

	err := command.Run()
	if err != nil {
		return fmt.Sprintf("Error: %s\nOutput: %s", err, out.String())
	}

	if out.String() == "" {
		return "Executed successfully (No Output)."
	}
	return out.String()
}
