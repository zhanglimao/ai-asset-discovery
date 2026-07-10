package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-asset-discovery/internal/model"
)

func TestFileScanner_MatchFiles(t *testing.T) {
	fs := NewFileScanner()

	// Create a temporary directory with a test config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".test-config.json")
	if err := os.WriteFile(configPath, []byte(`{"model": "gpt-4", "version": "1.0"}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rule := model.AgentRule{
		Name:          "test-agent",
		DisplayName:   "Test Agent",
		MinConfidence: "possible",
		Files: []model.FileRule{
			{
				Path:     filepath.Join(dir, ".test-config.json"),
				FileType: "file",
				Required: true,
				Contains: "gpt-4",
			},
		},
	}

	result := fs.matchFiles(rule)
	if result == nil {
		t.Fatal("expected match for existing file")
	}

	if result.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", result.Name, "test-agent")
	}
	if len(result.Files) != 1 {
		t.Errorf("len(Files) = %d, want 1", len(result.Files))
	}
}

func TestFileScanner_MatchFiles_Directory(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	rule := model.AgentRule{
		Name:          "test-agent",
		DisplayName:   "Test Agent",
		MinConfidence: "possible",
		Files: []model.FileRule{
			{
				Path:     dir,
				FileType: "directory",
				Required: true,
			},
		},
	}

	result := fs.matchFiles(rule)
	if result == nil {
		t.Fatal("expected match for existing directory")
	}
}

func TestFileScanner_MatchFiles_MissingRequired(t *testing.T) {
	fs := NewFileScanner()

	rule := model.AgentRule{
		Name: "test-agent",
		Files: []model.FileRule{
			{
				Path:     "/tmp/nonexistent-file-xyzabc-123",
				FileType: "file",
				Required: true,
			},
		},
	}

	result := fs.matchFiles(rule)
	if result != nil {
		t.Error("expected nil result for missing required file")
	}
}

func TestFileScanner_MatchFiles_ContentMismatch(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"model": "claude"}`), 0644)

	rule := model.AgentRule{
		Name: "test-agent",
		Files: []model.FileRule{
			{
				Path:     configPath,
				FileType: "file",
				Required: true,
				Contains: "gpt-4", // won't match
			},
		},
	}

	result := fs.matchFiles(rule)
	if result != nil {
		t.Error("expected nil result when content doesn't match required file")
	}
}

func TestFileScanner_MatchFiles_OptionalMissing(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "exists.txt")
	os.WriteFile(existingPath, []byte("hello"), 0644)

	rule := model.AgentRule{
		Name: "test-agent",
		Files: []model.FileRule{
			{
				Path:     existingPath,
				FileType: "file",
				Required: true,
			},
			{
				Path:     "/tmp/nonexistent-optional-file",
				FileType: "file",
				Required: false,
			},
		},
	}

	result := fs.matchFiles(rule)
	if result == nil {
		t.Fatal("expected match - optional file missing should not fail")
	}
}

func TestFileScanner_Scan(t *testing.T) {
	fs := NewFileScanner()

	rules := []model.AgentRule{
		{
			Name: "has-files",
			Files: []model.FileRule{
				{
					Path:     "/etc/hostname",
					FileType: "file",
					Required: false,
				},
			},
		},
		{
			Name: "no-files",
			// No Files field -> should not appear
		},
	}

	results, err := fs.Scan(rules)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	// Should have at most one result (the one with files)
	if len(results) > 1 {
		t.Errorf("too many results: %d", len(results))
	}
}

func TestFileScanner_WalkDir(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(dir, "level1", "level2", "level3"), 0755)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(dir, "level1", "l1.txt"), []byte("l1"), 0644)
	os.WriteFile(filepath.Join(dir, "level1", "level2", "l2.txt"), []byte("l2"), 0644)
	os.WriteFile(filepath.Join(dir, "level1", "level2", "level3", "l3.txt"), []byte("l3"), 0644)

	// Max depth 1: should get all files at root level (depth=1)
	files, err := fs.walkDir(dir, 1)
	if err != nil {
		t.Fatalf("walkDir() error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("walkDir(depth=1) = %d files, want 2 (root.txt + l1.txt at depth 1): %v", len(files), files)
	}

	// Max depth 2: should get root.txt, l1.txt, l2.txt
	files, err = fs.walkDir(dir, 2)
	if err != nil {
		t.Fatalf("walkDir() error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("walkDir(depth=2) = %d files, want 3: %v", len(files), files)
	}
}

func TestFileScanner_MatchFiles_DirectoryWithMaxDepth(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"model": "gpt-4"}`), 0644)

	rule := model.AgentRule{
		Name:          "dir-agent",
		DisplayName:   "Dir Agent",
		MinConfidence: "possible",
		Files: []model.FileRule{
			{
				Path:     dir,
				FileType: "directory",
				Required: true,
				MaxDepth: 1,
			},
		},
	}

	result := fs.matchFiles(rule)
	if result == nil {
		t.Fatal("expected match for directory with max_depth")
	}
	if len(result.Files) == 0 {
		t.Error("expected at least one file evidence")
	}
}

func TestFileScanner_MatchFiles_DirectoryContentMatch(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(`model: gpt-4`), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte(`model: claude`), 0644)

	rule := model.AgentRule{
		Name:          "dir-content-agent",
		DisplayName:   "Dir Content Agent",
		MinConfidence: "possible",
		Files: []model.FileRule{
			{
				Path:     dir,
				FileType: "directory",
				Required: true,
				MaxDepth: 1,
				Contains: "gpt-4",
			},
		},
	}

	result := fs.matchFiles(rule)
	if result == nil {
		t.Fatal("expected match for directory with content filter")
	}
}

func TestFileScanner_MatchFiles_DirectoryContentNoMatch(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(`model: claude`), 0644)

	rule := model.AgentRule{
		Name: "dir-content-agent",
		Files: []model.FileRule{
			{
				Path:     dir,
				FileType: "directory",
				Required: true,
				MaxDepth: 1,
				Contains: "gpt-4", // won't match
			},
		},
	}

	result := fs.matchFiles(rule)
	if result != nil {
		t.Error("expected nil result when directory content doesn't match required filter")
	}
}

func TestFileScanner_MatchFiles_OSFilter(t *testing.T) {
	fs := NewFileScanner()

	rule := model.AgentRule{
		Name: "os-specific",
		Files: []model.FileRule{
			{
				Path:     "/etc/hostname",
				FileType: "file",
				Required: true,
				OS:       "windows", // should be skipped on Linux
			},
		},
	}

	result := fs.matchFiles(rule)
	if result != nil {
		t.Error("expected nil result for non-matching OS filter")
	}
}

func TestFileScanner_MatchFiles_FileTypeMismatch(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()

	// File exists but we expect a directory
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)

	rule := model.AgentRule{
		Name: "type-mismatch",
		Files: []model.FileRule{
			{
				Path:     filepath.Join(dir, "file.txt"),
				FileType: "directory",
				Required: true,
			},
		},
	}

	result := fs.matchFiles(rule)
	if result != nil {
		t.Error("expected nil result when file_type doesn't match")
	}
}

func TestFileScanner_CheckFileContent_Large(t *testing.T) {
	fs := NewFileScanner()

	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.json")
	// Create a file where the match string is within the first 10KB (before truncation)
	// Pad after the match string to exceed 10KB overall
	content := "gpt-4" + strings.Repeat("x", 20000)
	os.WriteFile(largePath, []byte(content), 0644)

	result, matched := fs.checkFileContent(largePath, "gpt-4")
	if !matched {
		t.Error("expected content match in large file")
	}
	if result == "" {
		t.Error("expected non-empty content result")
	}
}
