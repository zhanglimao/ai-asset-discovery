// Package platform provides platform-aware path resolution helpers.
// All IDE-specific knowledge belongs in rules (YAML), not in Go code.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// CurrentOS returns "linux", "darwin", or "windows".
func CurrentOS() string { return runtime.GOOS }

// AppConfigDir returns the platform-appropriate configuration directory
// for a given application name. This is the XDG-compliant or Windows
// convention equivalent.
//
//	Linux:   ~/.config/<appName>
//	macOS:   ~/Library/Application Support/<appName>
//	Windows: %APPDATA%/<appName>  or  %LOCALAPPDATA%/<appName>
func AppConfigDir(appName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, appName)
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", appName)
	default: // linux and others
		// XDG_CONFIG_HOME
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			return filepath.Join(xdgConfig, appName)
		}
		return filepath.Join(home, ".config", appName)
	}
}

// AppHomeDir returns the platform-appropriate dot-directory in the
// user's home. Most CLI tools use ~/.<appName> on all platforms.
//
//	Linux/macOS: ~/.<appName>
//	Windows:     %USERPROFILE%\.<appName>
func AppHomeDir(appName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "."+appName)
}
