package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-asset-discovery/internal/model"
)

func TestEngine_LoadRulesFromBytes(t *testing.T) {
	engine := NewEngine()

	yamlData := []byte(`
version: "1.0"
agents:
  - name: test-agent
    display_name: "Test Agent"
    description: "For testing"
    category: "test"
`)

	err := engine.LoadRulesFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	if engine.rules == nil {
		t.Fatal("rules is nil after loading")
	}
	if len(engine.rules.Agents) != 1 {
		t.Errorf("len(Agents) = %d, want 1", len(engine.rules.Agents))
	}
}

func TestEngine_Run_NoRules(t *testing.T) {
	engine := NewEngine()
	_, err := engine.Run()
	if err == nil {
		t.Error("expected error when no rules loaded")
	}
}

func TestEngine_Run_WithFileScan(t *testing.T) {
	engine := NewEngine()

	// Create a temp dir with a file that can be detected
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, ".test-agent-config")
	os.WriteFile(testFile, []byte("test content"), 0644)

	yamlData := []byte(`
version: "1.0"
agents:
  - name: file-test-agent
    display_name: "File Test Agent"
    description: "Test file-based detection"
    category: "test"
    min_confidence: possible
    files:
      - path: ` + testFile + `
        file_type: file
        required: true
`)

	err := engine.LoadRulesFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", result.Version)
	}

	// Should find at least the file-based agent
	found := false
	for _, a := range result.Agents {
		if a.Name == "file-test-agent" {
			found = true
			if a.Confidence != "possible" {
				t.Errorf("Confidence = %q, want possible", a.Confidence)
			}
			break
		}
	}
	if !found {
		t.Error("file-test-agent not found in results")
	}
}

func TestEngine_Run_WithSkills(t *testing.T) {
	engine := NewEngine()

	// Create temp skill files
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillDir, 0755)
	// Skill file must exceed 1KB minimum size
	os.WriteFile(filepath.Join(skillDir, "test-skill.md"),
		[]byte("---\nname: test-skill\ndescription: Test skill with padding to meet minimum size requirement\n---\n# Test\n"+
			"This is a skill for testing purposes.\n"+
			"It contains enough content to exceed the 1KB minimum file size filter.\n"+
			strings.Repeat("This is padding to reach the required minimum file size for skill detection.\n", 20)), 0644)

	yamlData := []byte(`
version: "1.0"
agents:
  - name: skill-agent
    display_name: "Skill Agent"
    description: "Agent with skills"
    category: "test"
    min_confidence: possible
    files:
      - path: ` + skillDir + `
        file_type: directory
        required: true
    skills:
      enabled: true
      scan_paths:
        - ` + skillDir + `
      extensions:
        - .md
      keywords:
        - skill
`)

	err := engine.LoadRulesFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Find the skill agent
	var skillAgent *model.DiscoveredAgent
	for i := range result.Agents {
		if result.Agents[i].Name == "skill-agent" {
			skillAgent = &result.Agents[i]
			break
		}
	}

	if skillAgent == nil {
		t.Fatal("skill-agent not found")
	}

	if len(skillAgent.Skills) < 1 {
		t.Errorf("expected at least 1 skill, got %d", len(skillAgent.Skills))
	}
}

func TestEngine_Run_ConfigExtraction(t *testing.T) {
	engine := NewEngine()

	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := map[string]any{
		"llm": map[string]any{
			"model":    "gpt-4",
			"api_base": "https://api.openai.com",
		},
		"version": "1.5.0",
	}
	configBytes, _ := json.Marshal(configData)
	os.WriteFile(configPath, configBytes, 0644)

	yamlData := []byte(`
version: "1.0"
agents:
  - name: config-test-agent
    display_name: "Config Test"
    description: "Test config extraction"
    category: "test"
    min_confidence: possible
    files:
      - path: ` + configPath + `
        file_type: file
        required: true
    config:
      format: json
      paths:
        - ` + configPath + `
      field_map:
        model: "llm.model"
        api_url: "llm.api_base"
        version: "version"
`)

	err := engine.LoadRulesFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Find config agent
	var found *model.DiscoveredAgent
	for i := range result.Agents {
		if result.Agents[i].Name == "config-test-agent" {
			found = &result.Agents[i]
			break
		}
	}

	if found == nil {
		t.Fatal("config-test-agent not found")
	}

	if found.Config == nil {
		t.Fatal("Config is nil")
	}

	if model, ok := found.Config["model"]; !ok || model != "gpt-4" {
		t.Errorf("Config[model] = %v, want gpt-4", found.Config["model"])
	}
	if apiURL, ok := found.Config["api_url"]; !ok || apiURL != "https://api.openai.com" {
		t.Errorf("Config[api_url] = %v", found.Config["api_url"])
	}
	if found.Version != "1.5.0" {
		t.Errorf("Version = %q, want 1.5.0", found.Version)
	}
}

func TestDeduplicateAgents(t *testing.T) {
	agents := []model.DiscoveredAgent{
		{Name: "agent-a", AssetType: "file", Confidence: "possible"},
		{Name: "agent-a", AssetType: "file", Confidence: "confirmed"}, // higher confidence
		{Name: "agent-b", AssetType: "process", Confidence: "possible"},
		{Name: "agent-a", AssetType: "process", Confidence: "ghost"}, // different type
	}

	result := deduplicateAgents(agents)

	// Should have 3 unique name:type combinations
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	// agent-a:file should have "confirmed" confidence
	for _, a := range result {
		if a.Name == "agent-a" && a.AssetType == "file" {
			if a.Confidence != "confirmed" {
				t.Errorf("agent-a file confidence = %q, want confirmed", a.Confidence)
			}
		}
	}
}

func TestConfidenceRank(t *testing.T) {
	tests := []struct {
		c    model.Confidence
		want int
	}{
		{"confirmed", 3},
		{"possible", 2},
		{"ghost", 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		got := confidenceRank(tt.c)
		if got != tt.want {
			t.Errorf("confidenceRank(%q) = %d, want %d", tt.c, got, tt.want)
		}
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]any{
		"llm": map[string]any{
			"model": "gpt-4",
		},
		"name": "test",
	}

	tests := []struct {
		path string
		want any
	}{
		{"llm.model", "gpt-4"},
		{"name", "test"},
		{"llm.provider", nil},
		{"nonexistent.path", nil},
	}

	for _, tt := range tests {
		got := getNestedValue(data, tt.path)
		if got != tt.want {
			t.Errorf("getNestedValue(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSummaryPopulation(t *testing.T) {
	engine := NewEngine()

	result := &Result{
		Version: "1.0",
		Summary: Summary{
			ByType: make(map[string]int),
		},
		Agents: []model.DiscoveredAgent{
			{Name: "a", Confidence: "confirmed", AssetType: "file", Skills: []model.Skill{{Name: "s1"}, {Name: "s2"}}},
			{Name: "b", Confidence: "possible", AssetType: "process", Skills: []model.Skill{{Name: "s3"}}},
			{Name: "c", Confidence: "ghost", AssetType: "file"},
		},
	}

	engine.populateSummary(result, nil)

	s := result.Summary
	if s.TotalAgents != 3 {
		t.Errorf("TotalAgents = %d, want 3", s.TotalAgents)
	}
	if s.ConfirmedAgents != 1 {
		t.Errorf("ConfirmedAgents = %d, want 1", s.ConfirmedAgents)
	}
	if s.PossibleAgents != 1 {
		t.Errorf("PossibleAgents = %d, want 1", s.PossibleAgents)
	}
	if s.GhostAgents != 1 {
		t.Errorf("GhostAgents = %d, want 1", s.GhostAgents)
	}
	if s.TotalSkills != 3 {
		t.Errorf("TotalSkills = %d, want 3", s.TotalSkills)
	}
	if s.ByType["file"] != 2 {
		t.Errorf("ByType[file] = %d, want 2", s.ByType["file"])
	}
	if s.ByType["process"] != 1 {
		t.Errorf("ByType[process] = %d, want 1", s.ByType["process"])
	}
}

func TestParseSimpleTOML(t *testing.T) {
	content := `
# Comment line
title = "My Config"

[server]
port = "8080"
host = "localhost"

[db]
url = "postgres://localhost"
`
	result := parseSimpleTOML(content)

	if result["title"] != "My Config" {
		t.Errorf("title = %q, want My Config", result["title"])
	}

	server, ok := result["server"].(map[string]any)
	if !ok {
		t.Fatal("server section not found")
	}
	if server["port"] != "8080" {
		t.Errorf("server.port = %q, want 8080", server["port"])
	}
	if server["host"] != "localhost" {
		t.Errorf("server.host = %q, want localhost", server["host"])
	}

	db, ok := result["db"].(map[string]any)
	if !ok {
		t.Fatal("db section not found")
	}
	if db["url"] != "postgres://localhost" {
		t.Errorf("db.url = %q, want postgres://localhost", db["url"])
	}
}

func TestParseSimpleTOML_Empty(t *testing.T) {
	result := parseSimpleTOML("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseEnvContent(t *testing.T) {
	content := `
# Comment
API_KEY=sk-1234
API_BASE=https://api.openai.com
MODEL=gpt-4
`
	result := parseEnvContent(content)
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}
	if result["API_KEY"] != "sk-1234" {
		t.Errorf("API_KEY = %q, want sk-1234", result["API_KEY"])
	}
	if result["API_BASE"] != "https://api.openai.com" {
		t.Errorf("API_BASE = %q", result["API_BASE"])
	}
	if result["MODEL"] != "gpt-4" {
		t.Errorf("MODEL = %q, want gpt-4", result["MODEL"])
	}
}

func TestParseEnvContent_Empty(t *testing.T) {
	result := parseEnvContent("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseConfigFormat(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		format  string
		wantKey string
		wantVal string
	}{
		{"json", `{"name": "test"}`, "json", "name", "test"},
		{"yaml", "name: test", "yaml", "name", "test"},
		{"yml", "name: test", "yml", "name", "test"},
		{"toml", "name = \"test\"", "toml", "name", "test"},
		{"env", "NAME=test", "env", "NAME", "test"},
		{"unknown", "whatever", "unknown", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConfigFormat([]byte(tt.data), tt.format)
			if tt.wantKey == "" {
				if len(result) != 0 {
					t.Errorf("expected empty result, got %d entries", len(result))
				}
				return
			}
			if result[tt.wantKey] != tt.wantVal {
				t.Errorf("result[%q] = %v, want %v", tt.wantKey, result[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestDeduplicateAgents_SkillsMerge(t *testing.T) {
	agents := []model.DiscoveredAgent{
		{
			Name: "agent-a", AssetType: "process", Confidence: "possible",
			Skills: []model.Skill{{Name: "s1"}},
		},
		{
			Name: "agent-a", AssetType: "process", Confidence: "ghost",
			Skills: []model.Skill{{Name: "s2"}},
		},
	}

	result := deduplicateAgents(agents)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	// Skills from the lower-confidence entry should be merged into the higher one
	if len(result[0].Skills) != 2 {
		t.Errorf("len(Skills) = %d, want 2 (merged)", len(result[0].Skills))
	}
}

func TestGetNestedValue_Nil(t *testing.T) {
	got := getNestedValue(nil, "any.path")
	if got != nil {
		t.Errorf("getNestedValue(nil) = %v, want nil", got)
	}
}

// =========================================================================
// End-to-End Integration Tests (Engine Level)
// =========================================================================

// TestE2E_FileEvidenceProducesResult tests that a file-evidenced agent
// is discovered even when no matching process is running.
func TestE2E_FileEvidenceProducesResult(t *testing.T) {
	engine := NewEngine()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".test-agent-config")
	os.WriteFile(configFile, []byte("config-data"), 0644)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: test-cli-agent
    display_name: "Test CLI Agent"
    description: "A test CLI-based AI agent"
    category: "cli_agent"
    min_confidence: possible
    files:
      - path: ` + configFile + `
        file_type: file
        required: false
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, a := range result.Agents {
		if a.Name == "test-cli-agent" {
			found = true
			if a.AssetType != model.AssetTypeFile {
				t.Errorf("AssetType = %q, want file (from file evidence)", a.AssetType)
			}
			if len(a.Files) < 1 {
				t.Error("expected at least 1 file evidence")
			}
			if a.Files[0].Path != configFile {
				t.Errorf("File path = %q, want %q", a.Files[0].Path, configFile)
			}
			break
		}
	}
	if !found {
		t.Error("test-cli-agent not found in results")
	}
}

// TestE2E_IDEExtensionAgentDetection tests IDE extension scanning
// with agent capability detection — verifying that plugin-type agents
// are correctly detected and their agent capabilities recognized.
func TestE2E_IDEExtensionAgentDetection(t *testing.T) {
	engine := NewEngine()

	extDir := t.TempDir()
	copilotDir := filepath.Join(extDir, "github.copilot-1.0.0")
	os.MkdirAll(copilotDir, 0755)

	manifest := map[string]any{
		"name":        "copilot",
		"displayName": "GitHub Copilot",
		"version":     "1.0.0",
		"publisher":   "github",
		"description": "AI pair programmer",
		"categories":  []string{"Machine Learning", "AI"},
		"keywords":    []string{"copilot", "ai", "autocomplete", "agent"},
		"contributes": map[string]any{
			"agent": map[string]any{"type": "chat"},
		},
		"activationEvents": []string{"onChat:agent", "*"},
		"main":             "dist/agent.js",
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(copilotDir, "package.json"), data, 0644)
	os.MkdirAll(filepath.Join(copilotDir, "dist"), 0755)
	os.WriteFile(filepath.Join(copilotDir, "dist/agent.js"),
		[]byte(`module.exports = { createAgent: function() {} }`), 0644)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: github-copilot
    display_name: "GitHub Copilot"
    description: "GitHub Copilot AI coding assistant"
    category: "ide_extension"
    min_confidence: possible
    ide:
      paths:
        - ` + extDir + `
      ext_ids:
        - "github.copilot"

  - name: github-copilot-agent
    display_name: "GitHub Copilot Agent"
    description: "GitHub Copilot with agent mode enabled"
    category: "ide_extension"
    min_confidence: confirmed
    ide:
      paths:
        - ` + extDir + `
      ext_ids:
        - "github.copilot"
      agent_signals:
        - "createAgent"
        - "agent"
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	copilotFound := false
	copilotAgentFound := false
	for _, a := range result.Agents {
		if a.Name == "github-copilot" {
			copilotFound = true
			if a.AssetType != model.AssetTypeIDEExtension {
				t.Errorf("github-copilot AssetType = %q, want ide_extension", a.AssetType)
			}
		}
		if a.Name == "github-copilot-agent" {
			copilotAgentFound = true
			if a.AssetType != model.AssetTypeIDEExtension {
				t.Errorf("github-copilot-agent AssetType = %q, want ide_extension", a.AssetType)
			}
			if a.Confidence != model.ConfidenceConfirmed {
				t.Errorf("github-copilot-agent Confidence = %q, want confirmed (signals: %v)",
					a.Confidence, a.Extension.AgentSignals)
			}
			if a.Extension == nil {
				t.Fatal("Extension is nil")
			}
			if !a.Extension.HasAgent {
				t.Error("HasAgent should be true")
			}
			if len(a.Extension.AgentSignals) == 0 {
				t.Error("AgentSignals should not be empty")
			}
		}
	}
	if !copilotFound {
		t.Error("github-copilot not found")
	}
	if !copilotAgentFound {
		t.Error("github-copilot-agent not found (plugin-type agent detection)")
	}
}

// TestE2E_OSFilteredFileRules tests that OS:all file rules work on any platform.
func TestE2E_OSFilteredFileRules(t *testing.T) {
	engine := NewEngine()

	tmpDir := t.TempDir()
	allFile := filepath.Join(tmpDir, "all-config.json")
	os.WriteFile(allFile, []byte(`{}`), 0644)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: cross-platform-agent
    display_name: "Cross Platform"
    description: "Detectable on all OSes"
    category: "cli_agent"
    min_confidence: possible
    files:
      - path: ` + allFile + `
        file_type: file
        required: true
        os: all
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, a := range result.Agents {
		if a.Name == "cross-platform-agent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cross-platform-agent should be found (os: all)")
	}
}

// TestE2E_ProcessAndFileDedup verifies merging of same agent from
// process + file scanning paths.
func TestE2E_ProcessAndFileDedup(t *testing.T) {
	engine := NewEngine()

	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, ".myagent")
	os.MkdirAll(evidenceDir, 0755)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: my-agent
    display_name: "My Agent"
    description: "Test agent"
    category: "cli_agent"
    min_confidence: possible
    process:
      match_logic: or
      name_patterns:
        - type: contains
          value: "my-agent"
          weight: 10
    files:
      - path: ` + evidenceDir + `
        file_type: directory
        required: false
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	count := 0
	for _, a := range result.Agents {
		if a.Name == "my-agent" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected ≤1 my-agent entries after dedup, got %d", count)
	}
}

// TestE2E_NonAIExtensionNotMatched verifies a non-AI extension
// (like ms.python) is NOT matched when ExtIDs filter is active.
func TestE2E_NonAIExtensionNotMatched(t *testing.T) {
	engine := NewEngine()

	extDir := t.TempDir()
	pythonDir := filepath.Join(extDir, "ms.python-1.0.0")
	os.MkdirAll(pythonDir, 0755)
	pythonManifest := map[string]any{
		"name":        "python",
		"displayName": "Python",
		"version":     "1.0.0",
		"publisher":   "ms",
		"description": "Python language support",
	}
	pythonData, _ := json.Marshal(pythonManifest)
	os.WriteFile(filepath.Join(pythonDir, "package.json"), pythonData, 0644)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: github-copilot
    display_name: "GitHub Copilot"
    min_confidence: possible
    ide:
      paths:
        - ` + extDir + `
      ext_ids:
        - "github.copilot"
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, a := range result.Agents {
		if a.Name == "github-copilot" {
			t.Error("github-copilot should NOT match ms.python extension (ExtIDs filter)")
		}
	}
}

// TestE2E_RequiredFileMissingBlocksDetection verifies that a required
// file being absent blocks the entire agent detection.
func TestE2E_RequiredFileMissingBlocksDetection(t *testing.T) {
	engine := NewEngine()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.json")
	os.WriteFile(existingFile, []byte("ok"), 0644)
	missingFile := filepath.Join(tmpDir, "does-not-exist.json")

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: partial-evidence-agent
    display_name: "Partial Evidence"
    min_confidence: possible
    files:
      - path: ` + existingFile + `
        file_type: file
        required: false
      - path: ` + missingFile + `
        file_type: file
        required: true
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, a := range result.Agents {
		if a.Name == "partial-evidence-agent" {
			t.Error("partial-evidence-agent should not be detected (required file missing)")
		}
	}
}

// TestE2E_ExplicitPathsIDEScan verifies IDE scanning via explicit paths.
func TestE2E_ExplicitPathsIDEScan(t *testing.T) {
	engine := NewEngine()

	extDir := t.TempDir()
	copilotDir := filepath.Join(extDir, "github.copilot-1.0.0")
	os.MkdirAll(copilotDir, 0755)

	manifest := map[string]any{
		"name":        "copilot",
		"displayName": "GitHub Copilot",
		"version":     "1.0.0",
		"publisher":   "github",
		"description": "AI coding assistant",
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(copilotDir, "package.json"), data, 0644)

	rulesYAML := []byte(`
version: "1.0"
agents:
  - name: copilot-ext
    display_name: "Copilot Ext"
    min_confidence: possible
    ide:
      paths:
        - ` + extDir + `
      ext_ids:
        - "github.copilot"
`)

	if err := engine.LoadRulesFromBytes(rulesYAML); err != nil {
		t.Fatalf("LoadRulesFromBytes() error: %v", err)
	}

	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, a := range result.Agents {
		if a.Name == "copilot-ext" {
			found = true
			if a.Extension == nil {
				t.Fatal("Extension is nil")
			}
			if a.Extension.ID != "github.copilot" {
				t.Errorf("Extension ID = %q, want github.copilot", a.Extension.ID)
			}
		}
	}
	if !found {
		t.Error("copilot-ext not found")
	}
}
