package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Install() error {
	switch runtime.GOOS {
	case "linux":
		return m.installLinuxService()
	case "windows":
		return m.installWindowsService()
	case "darwin":
		return m.installMacOSService()
	default:
		return fmt.Errorf("service installation not supported on %s", runtime.GOOS)
	}
}

func (m *Manager) Uninstall() error {
	switch runtime.GOOS {
	case "linux":
		return m.uninstallLinuxService()
	case "windows":
		return m.uninstallWindowsService()
	case "darwin":
		return m.uninstallMacOSService()
	default:
		return fmt.Errorf("service uninstallation not supported on %s", runtime.GOOS)
	}
}

func (m *Manager) IsInstalled() bool {
	switch runtime.GOOS {
	case "linux":
		return m.isLinuxServiceInstalled()
	case "windows":
		return m.isWindowsServiceInstalled()
	case "darwin":
		return m.isMacOSServiceInstalled()
	default:
		return false
	}
}

// Linux systemd service management
func (m *Manager) installLinuxService() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=MySQL Backup System
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s -daemon
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, os.Getenv("USER"), execPath)

	serviceFile := "/etc/systemd/system/mysql-backup.service"
	
	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd and enable service
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := exec.Command("systemctl", "enable", "mysql-backup").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func (m *Manager) uninstallLinuxService() error {
	// Stop and disable service
	exec.Command("systemctl", "stop", "mysql-backup").Run()
	exec.Command("systemctl", "disable", "mysql-backup").Run()

	// Remove service file
	serviceFile := "/etc/systemd/system/mysql-backup.service"
	if err := os.Remove(serviceFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	return exec.Command("systemctl", "daemon-reload").Run()
}

func (m *Manager) isLinuxServiceInstalled() bool {
	serviceFile := "/etc/systemd/system/mysql-backup.service"
	_, err := os.Stat(serviceFile)
	return err == nil
}

// Windows service management
func (m *Manager) installWindowsService() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command("sc", "create", "MySQLBackup",
		"binPath=", fmt.Sprintf("\"%s\" -daemon", execPath),
		"start=", "auto",
		"DisplayName=", "MySQL Backup System",
	)

	return cmd.Run()
}

func (m *Manager) uninstallWindowsService() error {
	// Stop service first
	exec.Command("sc", "stop", "MySQLBackup").Run()
	
	// Delete service
	return exec.Command("sc", "delete", "MySQLBackup").Run()
}

func (m *Manager) isWindowsServiceInstalled() bool {
	cmd := exec.Command("sc", "query", "MySQLBackup")
	return cmd.Run() == nil
}

// macOS launchd service management
func (m *Manager) installMacOSService() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mysql-backup.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>-daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`, execPath)

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistFile := filepath.Join(launchAgentsDir, "com.mysql-backup.service.plist")
	if err := os.WriteFile(plistFile, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load the service
	return exec.Command("launchctl", "load", plistFile).Run()
}

func (m *Manager) uninstallMacOSService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistFile := filepath.Join(homeDir, "Library", "LaunchAgents", "com.mysql-backup.service.plist")
	
	// Unload the service
	exec.Command("launchctl", "unload", plistFile).Run()
	
	// Remove plist file
	if err := os.Remove(plistFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

func (m *Manager) isMacOSServiceInstalled() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	plistFile := filepath.Join(homeDir, "Library", "LaunchAgents", "com.mysql-backup.service.plist")
	_, err = os.Stat(plistFile)
	return err == nil
}
