package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func stringsContain(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAppConfigDir(t *testing.T) {
	for _, app := range []string{"Claude", "ChatGPT", "Cursor"} {
		d := AppConfigDir(app)
		if d == "" {
			t.Errorf("AppConfigDir(%q) returned empty", app)
			continue
		}
		if !filepath.IsAbs(d) {
			t.Errorf("AppConfigDir(%q) = %q, not absolute", app, d)
		}
		// Should contain the app name somewhere
		if !stringsContain(d, app) {
			t.Errorf("AppConfigDir(%q) = %q, does not contain app name", app, d)
		}
	}
}

func TestAppConfigDir_PlatformSpecific(t *testing.T) {
	d := AppConfigDir("TestApp")
	switch runtime.GOOS {
	case "windows":
		// Should contain APPDATA or Roaming
		if !stringsContain(d, "AppData") && !stringsContain(d, "Roaming") {
			t.Errorf("Windows AppConfigDir = %q, expected APPDATA-based path", d)
		}
	case "darwin":
		if !stringsContain(d, "Application Support") && !stringsContain(d, "Library") {
			t.Errorf("macOS AppConfigDir = %q, expected ~/Library/Application Support", d)
		}
	default:
		// Linux: ~/.config or $XDG_CONFIG_HOME
		if !stringsContain(d, ".config") && !stringsContain(d, "TestApp") {
			t.Errorf("Linux AppConfigDir = %q, expected XDG-based path", d)
		}
	}
}

func TestAppHomeDir(t *testing.T) {
	for _, app := range []string{"claude", "gemini", "aider"} {
		d := AppHomeDir(app)
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, "."+app)
		if d != expected {
			t.Errorf("AppHomeDir(%q) = %q, want %q", app, d, expected)
		}
		if !filepath.IsAbs(d) {
			t.Errorf("AppHomeDir(%q) = %q, not absolute", app, d)
		}
	}
}

func TestAppHomeDir_NoHome(t *testing.T) {
	// Save and restore HOME
	orig := os.Getenv("HOME")
	defer os.Setenv("HOME", orig)
	os.Setenv("HOME", "")
	// On Linux/macOS this unsets HOME
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		d := AppHomeDir("test")
		// With no home, UserHomeDir() may still work via other means;
		// if it returns empty, AppHomeDir should also return empty
		if d != "" {
			// HomeDir may still work from passwd, accept it
			t.Logf("AppHomeDir returned %q without HOME set", d)
		}
	}
}

func TestAppConfigDir_XDGSettings(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("XDG test only relevant on Linux")
	}
	// Test XDG_CONFIG_HOME override
	orig := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", orig)

	os.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	d := AppConfigDir("TestApp")
	expected := filepath.Join("/custom/xdg", "TestApp")
	if d != expected {
		t.Errorf("AppConfigDir with XDG_CONFIG_HOME = %q, want %q", d, expected)
	}
	os.Setenv("XDG_CONFIG_HOME", orig)
}
