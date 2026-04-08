//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
)

// executeShellCommand for Windows (Ghost Mode Enabled)
func executeShellCommand(cmd string) string {
	var command *exec.Cmd
	var out bytes.Buffer

	// Use cmd /C to execute
	command = exec.Command("cmd", "/C", cmd)

	// --- GHOST MODE (Windows Only) ---
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

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
