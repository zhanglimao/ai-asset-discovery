// Package platform provides platform-aware path resolution for known
// application directories (IDE extensions, config dirs, etc.).
// Instead of hardcoding paths like ~/.vscode/extensions in rules,
// scanners should first try the platform-specific auto-discovery
// functions here, and only fall back to rule-specified paths.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// IDE represents a known IDE type for extension directory discovery.
type IDE string

const (
	IDEVSCode   IDE = "vscode"
	IDECursor   IDE = "cursor"
	IDEWindsurf IDE = "windsurf"
)

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

// ExtensionsDirs returns the platform-appropriate extension directories
// for the given IDE type. If no known directories are found for this
// platform, an empty slice is returned (callers should fall back to
// rule-specified paths).
func ExtensionsDirs(ide IDE) []string {
	switch ide {
	case IDEVSCode:
		return vscodeExtensionsDirs()
	case IDECursor:
		return cursorExtensionsDirs()
	case IDEWindsurf:
		return windsurfExtensionsDirs()
	default:
		return nil
	}
}

func vscodeExtensionsDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var dirs []string

	switch runtime.GOOS {
	case "windows":
		// VS Code (user install)
		dirs = append(dirs, filepath.Join(home, ".vscode", "extensions"))
		// VS Code (system install) — extensions live under the app data
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dirs = append(dirs, filepath.Join(appData, "Code", "User"))
		// VS Code Insiders
		dirs = append(dirs, filepath.Join(home, ".vscode-insiders", "extensions"))
	case "darwin":
		dirs = append(dirs,
			filepath.Join(home, ".vscode", "extensions"),
		)
		// VS Code Server (remote SSH / container)
		dirs = append(dirs,
			filepath.Join(home, ".vscode-server", "extensions"),
		)
	default: // linux and others
		dirs = append(dirs,
			filepath.Join(home, ".vscode", "extensions"),
			filepath.Join(home, ".vscode-server", "extensions"),
		)
	}

	return dirs
}

func cursorExtensionsDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var dirs []string

	dirs = append(dirs,
		filepath.Join(home, ".cursor", "extensions"),
	)

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dirs = append(dirs, filepath.Join(appData, "Cursor", "extensions"))
	}

	return dirs
}

func windsurfExtensionsDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var dirs []string

	dirs = append(dirs,
		filepath.Join(home, ".windsurf", "extensions"),
	)

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dirs = append(dirs, filepath.Join(appData, "Windsurf", "extensions"))
	}

	return dirs
}
