package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"ai-monitor/internal/logger"
	"golang.org/x/sys/windows/registry"
)

const regKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "AI-Monitor"

// IsEnabled checks if the registry run entry points to this executable.
func IsEnabled() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("autostart: get executable: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return false, fmt.Errorf("autostart: resolve path: %w", err)
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false, nil // key may not exist = not enabled
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	if err != nil {
		return false, nil // value not set = not enabled
	}
	return val == abs, nil
}

// Enable creates a registry Run entry pointing to the current executable.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("autostart: get executable: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("autostart: resolve path: %w", err)
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("autostart: open registry: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue(appName, abs); err != nil {
		return fmt.Errorf("autostart: set value: %w", err)
	}
	logger.Info("[autostart] enabled: %s", abs)
	return nil
}

// Disable removes the registry Run entry.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.WRITE)
	if err != nil {
		return nil // key doesn't exist = nothing to clean
	}
	defer k.Close()

	if err := k.DeleteValue(appName); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("autostart: delete value: %w", err)
		}
	}
	logger.Info("[autostart] disabled")
	return nil
}
