package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// ExpandPath expands path variables like ~/, %VAR%, and {{VAR}}.
func ExpandPath(path string) (string, error) {
	// Handle ~/
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	// Expand {{VAR}} placeholders (used by platform ScanPath presets)
	path = expandDoubleBraces(path)

	// Handle %VAR% style (Windows-like) on all platforms
	if strings.Contains(path, "%") {
		path = expandEnvVars(path)
	}

	return filepath.Clean(path), nil
}

var doubleBraceResolvers = map[string]func() (string, error){
	"AppData": func() (string, error) {
		v := os.Getenv("APPDATA")
		if v == "" && runtime.GOOS == "windows" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			v = filepath.Join(home, "AppData", "Roaming")
		}
		return v, nil
	},
}

func expandDoubleBraces(s string) string {
	for k, f := range doubleBraceResolvers {
		pat := "{{" + k + "}}"
		if !strings.Contains(s, pat) {
			continue
		}
		v, err := f()
		if err != nil {
			continue
		}
		s = strings.ReplaceAll(s, pat, v)
	}
	return s
}

func expandEnvVars(path string) string {
	replacer := make([]string, 0)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			replacer = append(replacer, "%"+parts[0]+"%", parts[1])
		}
	}
	r := strings.NewReplacer(replacer...)
	return r.Replace(path)
}

// ResolveSkillPaths resolves skill scan paths, expanding variables.
func ResolveSkillPaths(paths []string) ([]string, error) {
	var resolved []string
	for _, p := range paths {
		expanded, err := ExpandPath(p)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, expanded)
	}
	return resolved, nil
}

// GetUser returns current username.
func GetUser() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

// IsLinux returns true on Linux.
func IsLinux() bool { return runtime.GOOS == "linux" }

// IsDarwin returns true on macOS.
func IsDarwin() bool { return runtime.GOOS == "darwin" }

// IsWindows returns true on Windows.
func IsWindows() bool { return runtime.GOOS == "windows" }

// OSName returns the current OS name.
func OSName() string { return runtime.GOOS }

// FilterByOS filters a list of OS-specific strings to those matching the current OS or "all".
func FilterByOS(items []string, osMapper func(string) string) []string {
	var result []string
	currentOS := runtime.GOOS
	for _, item := range items {
		osTarget := osMapper(item)
		if osTarget == "all" || osTarget == currentOS {
			result = append(result, item)
		}
	}
	return result
}
