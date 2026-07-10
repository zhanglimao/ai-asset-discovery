package ide

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/model"
)

// createTestExtension creates a directory with a package.json for testing.
func createTestExtension(t *testing.T, baseDir, publisher, name, displayName string, extra map[string]any, dirs ...string) string {
	t.Helper()
	extDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	manifest := map[string]any{
		"name":        name,
		"displayName": displayName,
		"version":     "1.0.0",
		"publisher":   publisher,
		"description": "A test extension",
	}
	for k, v := range extra {
		manifest[k] = v
	}

	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create any subdirectories needed
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(extDir, d), 0755)
	}

	return extDir
}

func TestScanner_Scan_NoIDERules(t *testing.T) {
	s := NewScanner()
	rules := []model.AgentRule{
		{Name: "no-ide", DisplayName: "No IDE"},
	}
	results, err := s.Scan(rules)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanner_Scan_ExtIDMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", nil)
	createTestExtension(t, extIDEDir, "ms", "python", "Python", nil)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "github-copilot" {
		t.Errorf("Name = %q, want github-copilot", results[0].Name)
	}
	if results[0].Extension == nil {
		t.Fatal("Extension is nil")
	}
	if !results[0].Extension.IsAI {
		t.Error("Extension.IsAI should be true")
	}
}

func TestScanner_Scan_ExtIDGlob(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "github", "copilot-chat", "GitHub Copilot Chat", nil)

	rule := model.AgentRule{
		Name:          "copilot-chat",
		DisplayName:   "Copilot Chat",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot*"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestScanner_Scan_ExtIDNoMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "ms", "python", "Python", nil)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanner_Scan_KeywordMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "ai-helper", "AI Helper", map[string]any{
		"categories": []string{"Machine Learning", "AI"},
	})

	rule := model.AgentRule{
		Name:          "ai-helper",
		DisplayName:   "AI Helper",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"ai"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestScanner_Scan_KeywordInExtensionKeywords(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "code-assist", "Code Assistant", map[string]any{
		"keywords": []string{"AI", "copilot", "chat"},
	})

	rule := model.AgentRule{
		Name:          "code-assist",
		DisplayName:   "Code Assistant",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestScanner_Scan_KeywordInDisplayName(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "my-ext", "AI Code Assistant", nil)

	rule := model.AgentRule{
		Name:          "ai-ext",
		DisplayName:   "AI Extension",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"ai"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestScanner_Scan_KeywordNoMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "theme-dark", "Dark Theme", map[string]any{
		"categories": []string{"Themes"},
	})

	rule := model.AgentRule{
		Name:          "ai-helper",
		DisplayName:   "AI Helper",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"ai"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanner_Scan_DependencyMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "chat-ext", "Chat Extension", map[string]any{
		"dependencies": map[string]string{
			"@anthropic/sdk": "^1.0.0",
		},
	})

	rule := model.AgentRule{
		Name:          "claude-ext",
		DisplayName:   "Claude Extension",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:   []string{extIDEDir},
			Depends: []string{"anthropic"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestScanner_Scan_DependencyNoMatch(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "lint-ext", "Lint Extension", map[string]any{
		"dependencies": map[string]string{
			"eslint": "^8.0.0",
		},
	})

	rule := model.AgentRule{
		Name:          "claude-ext",
		DisplayName:   "Claude Extension",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:   []string{extIDEDir},
			Depends: []string{"anthropic"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanner_Scan_CheckAgentCapability(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)

	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", map[string]any{
		"contributes": map[string]any{
			"agent": map[string]any{"type": "chat"},
		},
		"activationEvents": []string{"onChat:agent", "*"},
		"main":             "dist/agent.js",
	}, "dist", "skills")

	// Also write a minimal main file with an agent signal
	os.WriteFile(filepath.Join(extIDEDir, "copilot", "dist/agent.js"), []byte(`module.exports = { activate: function() {} }`), 0644)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:        []string{extIDEDir},
			ExtIDs:       []string{"github.copilot"},
			AgentSignals: []string{"activate"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Confidence != "confirmed" {
		t.Errorf("Confidence = %q, want confirmed", results[0].Confidence)
	}
	if results[0].Extension == nil {
		t.Fatal("Extension is nil")
	}
	if !results[0].Extension.HasAgent {
		t.Error("HasAgent should be true")
	}
	if len(results[0].Extension.AgentSignals) == 0 {
		t.Error("AgentSignals should not be empty")
	}
}

func TestScanner_Scan_CheckAgentCapability_NoAgent(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "someone", "simple-ext", "Simple Extension", nil)

	rule := model.AgentRule{
		Name:          "simple-ext",
		DisplayName:   "Simple Extension",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"someone.simple-ext"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Extension.HasAgent {
		t.Error("HasAgent should be false for simple extension")
	}
}

func TestScanner_Scan_ExtractConfigValue(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", map[string]any{
		"version": "1.99.0",
	})

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot"},
			ConfigKeys: []model.ConfigExtract{
				{Field: "version", KeyPath: "version"},
			},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Extension.Config["version"] != "1.99.0" {
		t.Errorf("Config[version] = %q, want 1.99.0", results[0].Extension.Config["version"])
	}
}

func TestScanner_Scan_ExtractConfigValue_Unknown(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", nil)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot"},
			ConfigKeys: []model.ConfigExtract{
				{Field: "nonexistent", KeyPath: "nonexistent.field"},
			},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Extension.Config["nonexistent"] != nil {
		t.Errorf("Config[nonexistent] should be nil, got %v", results[0].Extension.Config["nonexistent"])
	}
}

func TestScanner_Scan_NonExistentPath(t *testing.T) {
	s := NewScanner()
	rule := model.AgentRule{
		Name:          "test-agent",
		DisplayName:   "Test Agent",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{"/tmp/nonexistent-path-xyzabc"},
			ExtIDs: []string{"test.agent"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent path, got %d", len(results))
	}
}

func TestScanner_MatchGlob(t *testing.T) {
	tests := []struct {
		input   string
		pattern string
		want    bool
	}{
		{"github.copilot", "github.copilot", true},
		{"github.copilot", "github.copilot*", true},
		{"github.copilot-chat", "github.copilot*", true},
		{"ms.python", "github.*", false},
		{"github.copilot", "*copilot*", true},
		{"test", "*", true},
		{"test.ext", "test.*", true},
		{"GITHUB.COPILOT", "github.copilot", true}, // case insensitive
		{"", "", true},
		{"abc", "a*c", true},
		{"abc", "a*b*c", true},
		{"aXYZbXYZc", "a*b*c", true},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+tt.pattern, func(t *testing.T) {
			got := matchGlob(tt.input, tt.pattern)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.input, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestScanner_ReadManifest(t *testing.T) {
	s := NewScanner()
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "package.json")
	manifest := VSCodeExtensionManifest{
		Name:        "test-ext",
		DisplayName: "Test Extension",
		Version:     "2.0.0",
		Publisher:   "testpub",
		Description: "A test",
		Categories:  []string{"AI"},
		Keywords:    []string{"copilot"},
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(pkgPath, data, 0644)

	m, err := s.readManifest(pkgPath)
	if err != nil {
		t.Fatalf("readManifest() error: %v", err)
	}
	if m.Name != "test-ext" {
		t.Errorf("Name = %q, want test-ext", m.Name)
	}
	if m.DisplayName != "Test Extension" {
		t.Errorf("DisplayName = %q", m.DisplayName)
	}
}

func TestScanner_ReadManifest_Invalid(t *testing.T) {
	s := NewScanner()
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "package.json")
	os.WriteFile(pkgPath, []byte("not json"), 0644)

	_, err := s.readManifest(pkgPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestScanner_ReadManifest_NotFound(t *testing.T) {
	s := NewScanner()
	_, err := s.readManifest("/tmp/nonexistent-package-json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestScanner_Scan_KeywordOverExtIDPriority(t *testing.T) {
	// When ExtIDs are specified, keywords should NOT cause unintended matches
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)

	// Create an extension that has keyword "AI" but is NOT copilot
	createTestExtension(t, extIDEDir, "someone", "continue", "Continue", map[string]any{
		"categories": []string{"AI"},
	})

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot"}, // only match copilot
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	// Continue should NOT match even though it has AI keyword
	if len(results) != 0 {
		t.Errorf("expected 0 results (keyword should not override ExtID priority), got %d", len(results))
	}
}

func TestScanner_Scan_MultipleExtensions(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)

	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", nil)
	createTestExtension(t, extIDEDir, "github", "copilot-chat", "GitHub Copilot Chat", nil)
	createTestExtension(t, extIDEDir, "ms", "python", "Python", nil)

	rule := model.AgentRule{
		Name:          "copilot",
		DisplayName:   "Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:  []string{extIDEDir},
			ExtIDs: []string{"github.copilot", "github.copilot-chat"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestScanner_Scan_EmptyExtensionsDir(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	// No extensions in the directory

	rule := model.AgentRule{
		Name:          "test-agent",
		DisplayName:   "Test Agent",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"ai"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty dir, got %d", len(results))
	}
}

func TestScanner_Scan_DirWithNoPackageJSON(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode")
	os.MkdirAll(extIDEDir, 0755)
	// Create a directory without package.json
	os.MkdirAll(filepath.Join(extIDEDir, "broken-ext"), 0755)

	rule := model.AgentRule{
		Name:          "test-agent",
		DisplayName:   "Test Agent",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			Paths:    []string{extIDEDir},
			Keywords: []string{"ai"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanner_ResolveScanPaths_IDETypeVSCode(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{
		IDEType: "vscode",
	}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) == 0 {
		t.Fatal("expected non-empty paths for vscode ide_type")
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("path %q is not absolute", p)
		}
	}
	foundExt := false
	for _, p := range paths {
		if strings.Contains(filepath.ToSlash(p), "extensions") {
			foundExt = true
			break
		}
	}
	if !foundExt {
		t.Errorf("expected at least one path containing 'extensions', got %v", paths)
	}
}

func TestScanner_ResolveScanPaths_IDETypeWithFallback(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{
		IDEType: "vscode",
		Paths:   []string{"~/my-custom-extensions"},
	}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) == 0 {
		t.Fatal("expected non-empty paths")
	}
	foundCustom := false
	for _, p := range paths {
		if strings.Contains(p, "my-custom-extensions") {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Errorf("expected fallback custom path in results, got %v", paths)
	}
}

func TestScanner_ResolveScanPaths_OnlyExplicitPaths(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{
		Paths: []string{"/tmp/test-extensions"},
	}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) != 1 {
		t.Fatalf("expected exactly 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/tmp/test-extensions" {
		t.Errorf("path = %q, want /tmp/test-extensions", paths[0])
	}
}

func TestScanner_Scan_WithIDETypeAutoDiscovery(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	extIDEDir := filepath.Join(extDir, ".vscode", "extensions")
	os.MkdirAll(extIDEDir, 0755)
	createTestExtension(t, extIDEDir, "github", "copilot", "GitHub Copilot", nil)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", extDir)
	defer os.Setenv("HOME", origHome)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			IDEType: "vscode",
			ExtIDs:  []string{"github.copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "github-copilot" {
		t.Errorf("Name = %q, want github-copilot", results[0].Name)
	}
}

func TestScanner_Scan_WithIDETypeCursor(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	cursorExtDir := filepath.Join(extDir, ".cursor", "extensions")
	os.MkdirAll(cursorExtDir, 0755)
	createTestExtension(t, cursorExtDir, "github", "copilot-chat", "GitHub Copilot Chat", nil)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", extDir)
	defer os.Setenv("HOME", origHome)

	rule := model.AgentRule{
		Name:          "cursor-composer",
		DisplayName:   "Cursor Composer",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			IDEType: "cursor",
			ExtIDs:  []string{"github.copilot-chat"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Extension.IDEPath != cursorExtDir {
		t.Errorf("IDEPath = %q, want %q", results[0].Extension.IDEPath, cursorExtDir)
	}
}

func TestScanner_Scan_IDETypeFallbackToExplicitPath(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	fallbackDir := filepath.Join(extDir, "fallback-ext")
	os.MkdirAll(fallbackDir, 0755)
	createTestExtension(t, fallbackDir, "github", "copilot", "GitHub Copilot", nil)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			IDEType: "vscode",
			Paths:   []string{fallbackDir},
			ExtIDs:  []string{"github.copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result from fallback, got %d", len(results))
	}
}

func TestScanner_ResolveScanPaths_UnknownIDEType(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{
		IDEType: "unknown-ide",
		Paths:   []string{"/tmp/test-extensions"},
	}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) != 1 {
		t.Fatalf("expected 1 explicit path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/tmp/test-extensions" {
		t.Errorf("path = %q, want /tmp/test-extensions", paths[0])
	}
}

func TestScanner_ResolveScanPaths_IDETypeWindsurf(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{
		IDEType: "windsurf",
	}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) != 1 {
		t.Fatalf("expected 1 windsurf path, got %d: %v", len(paths), paths)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".windsurf", "extensions")
	if paths[0] != expected {
		t.Errorf("path = %q, want %q", paths[0], expected)
	}
}

func TestScanner_ResolveScanPaths_NoIDETypeNoPaths(t *testing.T) {
	s := NewScanner()
	ideRule := &model.IDERule{}
	paths := s.resolveScanPaths(ideRule)
	if len(paths) != 0 {
		t.Errorf("expected empty paths, got %v", paths)
	}
}

func TestScanner_Scan_IDETypeKeepsPathExpansion(t *testing.T) {
	s := NewScanner()
	extDir := t.TempDir()
	vscDir := filepath.Join(extDir, ".vscode", "extensions")
	os.MkdirAll(vscDir, 0755)
	createTestExtension(t, vscDir, "github", "copilot", "GitHub Copilot", nil)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", extDir)
	defer os.Setenv("HOME", origHome)

	rule := model.AgentRule{
		Name:          "github-copilot",
		DisplayName:   "GitHub Copilot",
		MinConfidence: "possible",
		IDE: &model.IDERule{
			IDEType: "vscode",
			Paths:   []string{"~/.my-backup-extensions"},
			ExtIDs:  []string{"github.copilot"},
		},
	}

	results, err := s.Scan([]model.AgentRule{rule})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result from auto-discovered VSCode path, got %d", len(results))
	}
}

func TestScanner_ResolveConfigPaths_ExpandPathCalled(t *testing.T) {
	s := NewScanner()
	paths := s.resolveScanPaths(&model.IDERule{IDEType: "vscode"})
	for _, p := range paths {
		expanded, err := config.ExpandPath(p)
		if err != nil {
			t.Errorf("ExpandPath(%q) error: %v", p, err)
			continue
		}
		if !filepath.IsAbs(expanded) {
			t.Errorf("ExpandPath(%q) = %q, not absolute", p, expanded)
		}
	}
}
