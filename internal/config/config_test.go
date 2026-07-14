package config

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestExpandPath_HomeDir(t *testing.T) {
	result, err := ExpandPath("~/test/path")
	if err != nil {
		t.Fatalf("ExpandPath() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := home + "/test/path"
	if result != expected {
		t.Errorf("ExpandPath() = %q, want %q", result, expected)
	}
}

func TestExpandPath_NoExpand(t *testing.T) {
	result, err := ExpandPath("/usr/local/bin")
	if err != nil {
		t.Fatalf("ExpandPath() error: %v", err)
	}
	if result != "/usr/local/bin" {
		t.Errorf("ExpandPath() = %q, want /usr/local/bin", result)
	}
}

func TestExpandPath_EnvVars(t *testing.T) {
	os.Setenv("TEST_VAR", "myapp")
	defer os.Unsetenv("TEST_VAR")

	result, err := ExpandPath("/opt/%TEST_VAR%/config")
	if err != nil {
		t.Fatalf("ExpandPath() error: %v", err)
	}
	if result != "/opt/myapp/config" {
		t.Errorf("ExpandPath() = %q, want /opt/myapp/config", result)
	}
}

func TestGetUser(t *testing.T) {
	user := GetUser()
	if user == "" || user == "unknown" {
		t.Skip("could not determine current user")
	}
}

func TestOSName(t *testing.T) {
	name := OSName()
	if name != runtime.GOOS {
		t.Errorf("OSName() = %q, want %q", name, runtime.GOOS)
	}
}

func TestIsLinux(t *testing.T) {
	if runtime.GOOS == "linux" && !IsLinux() {
		t.Error("IsLinux() should be true on Linux")
	}
}

func TestResolveSkillPaths(t *testing.T) {
	paths := []string{"~/skills", "/opt/skills"}
	resolved, err := ResolveSkillPaths(paths)
	if err != nil {
		t.Fatalf("ResolveSkillPaths() error: %v", err)
	}
	if len(resolved) != 2 {
		t.Errorf("len(resolved) = %d, want 2", len(resolved))
	}
}

func TestFilterByOS(t *testing.T) {
	// Test with a mapper that returns hardcoded OS values
	items := []string{"linux-item", "darwin-item", "all-item", "windows-item"}
	mapper := func(item string) string {
		switch item {
		case "linux-item":
			return "linux"
		case "darwin-item":
			return "darwin"
		case "windows-item":
			return "windows"
		case "all-item":
			return "all"
		default:
			return "unknown"
		}
	}

	result := FilterByOS(items, mapper)
	if len(result) == 0 {
		t.Error("FilterByOS returned empty, expected at least the all-item")
	}

	// The "all" item should always be included
	foundAll := false
	for _, r := range result {
		if r == "all-item" {
			foundAll = true
		}
	}
	if !foundAll {
		t.Error("all-item should always be included")
	}

	// Current OS item should be included
	currentOS := runtime.GOOS
	for _, r := range result {
		if r == currentOS+"-item" {
			return // found it
		}
	}
	t.Errorf("expected %s-item to be in results, got %v", currentOS, result)
}

func TestFilterByOS_Empty(t *testing.T) {
	result := FilterByOS(nil, func(s string) string { return s })
	if len(result) != 0 {
		t.Errorf("expected empty, got %d items", len(result))
	}
}

func TestExpandPath_AppDataStyle(t *testing.T) {
	os.Setenv("APPDATA", "/home/user/.config")
	defer os.Unsetenv("APPDATA")

	result, err := ExpandPath("%APPDATA%/myapp/config")
	if err != nil {
		t.Fatalf("ExpandPath() error: %v", err)
	}
	if result != "/home/user/.config/myapp/config" {
		t.Errorf("ExpandPath() = %q, want /home/user/.config/myapp/config", result)
	}
}

func TestIsDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" && !IsDarwin() {
		t.Error("IsDarwin() should be true on macOS")
	}
}

func TestIsWindows(t *testing.T) {
	if runtime.GOOS == "windows" && !IsWindows() {
		t.Error("IsWindows() should be true on Windows")
	}
}

func TestExpandPath_LocalAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("{{AppData}} / {{LocalAppData}} are Windows-oriented; skip on non-Windows")
	}

	// {{AppData}}
	result, err := ExpandPath("{{AppData}}/test/app")
	if err != nil {
		t.Fatalf("ExpandPath({{AppData}}) error: %v", err)
	}
	if !strings.Contains(result, "AppData") {
		t.Errorf("ExpandPath({{AppData}}) = %q, expected to contain AppData", result)
	}

	// {{LocalAppData}}
	result2, err := ExpandPath("{{LocalAppData}}/test/app")
	if err != nil {
		t.Fatalf("ExpandPath({{LocalAppData}}) error: %v", err)
	}
	// Should resolve to something with Local or AppData in path
	if !strings.Contains(result2, "AppData") && !strings.Contains(result2, "Local") {
		t.Errorf("ExpandPath({{LocalAppData}}) = %q, expected to contain Local or AppData", result2)
	}
}

func TestExpandPath_LocalAppData_NonWindowsFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows resolves {{LocalAppData}} from env; test non-Windows fallback")
	}

	// On non-Windows with LOCALAPPDATA unset, {{LocalAppData}} should not error
	result, err := ExpandPath("{{LocalAppData}}/test/app")
	if err != nil {
		t.Fatalf("ExpandPath({{LocalAppData}}) should not error on non-Windows: %v", err)
	}
	t.Logf("ExpandPath({{LocalAppData}}/test/app) on %s = %q", runtime.GOOS, result)
}
