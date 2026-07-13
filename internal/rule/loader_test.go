package rule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Parse(t *testing.T) {
	loader := NewLoader()

	yamlData := []byte(`
version: "1.0"
agents:
  - name: test-agent
    display_name: "Test Agent"
    description: "A test agent"
    category: "test"
    min_confidence: possible
    process:
      match_logic: or
      name_patterns:
        - type: exact
          value: "test-agent"
          weight: 10
  - name: another-agent
    display_name: "Another Agent"
    description: "Another test agent"
    category: "test"
    min_confidence: confirmed
`)

	rf, err := loader.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if rf.Version != "1.0" {
		t.Errorf("Version = %q, want %q", rf.Version, "1.0")
	}

	if len(rf.Agents) != 2 {
		t.Fatalf("len(Agents) = %d, want 2", len(rf.Agents))
	}

	// Check first agent
	a1 := rf.Agents[0]
	if a1.Name != "test-agent" {
		t.Errorf("Agent[0].Name = %q, want %q", a1.Name, "test-agent")
	}
	if a1.MinConfidence != "possible" {
		t.Errorf("Agent[0].MinConfidence = %q, want %q", a1.MinConfidence, "possible")
	}
	if a1.Process == nil {
		t.Fatal("Agent[0].Process is nil")
	}
	if a1.Process.MatchLogic != "or" {
		t.Errorf("Process.MatchLogic = %q, want %q", a1.Process.MatchLogic, "or")
	}

	// Check min_confidence defaults
	a2 := rf.Agents[1]
	if a2.MinConfidence != "confirmed" {
		t.Errorf("Agent[1].MinConfidence = %q, want %q", a2.MinConfidence, "confirmed")
	}
}

func TestLoader_Parse_WithSkillRule(t *testing.T) {
	loader := NewLoader()

	yamlData := []byte(`
version: "1.0"
agents:
  - name: skill-agent
    display_name: "Skill Agent"
    description: "An agent with skills"
    category: "test"
    skills:
      scan_paths:
        - /tmp/skills
`)

	rf, err := loader.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(rf.Agents) != 1 {
		t.Fatalf("len(Agents) = %d, want 1", len(rf.Agents))
	}

	sk := rf.Agents[0].Skills
	if sk == nil {
		t.Fatal("Skills is nil")
	}
	if sk.MaxDepth != 3 {
		t.Errorf("Skills.MaxDepth = %d, want 3 (default)", sk.MaxDepth)
	}
	if sk.MaxSizeKB != 100 {
		t.Errorf("Skills.MaxSizeKB = %d, want 100 (default)", sk.MaxSizeKB)
	}
}

func TestLoader_LoadFile(t *testing.T) {
	// Create a temporary YAML file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.yaml")

	data := []byte(`
version: "1.0"
agents:
  - name: file-agent
    display_name: "File Agent"
    description: "Loaded from file"
    category: "test"
`)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	loader := NewLoader()
	rf, err := loader.LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}

	if len(rf.Agents) != 1 {
		t.Fatalf("len(Agents) = %d, want 1", len(rf.Agents))
	}
	if rf.Agents[0].Name != "file-agent" {
		t.Errorf("Agent.Name = %q, want %q", rf.Agents[0].Name, "file-agent")
	}
}

func TestLoader_LoadDir(t *testing.T) {
	dir := t.TempDir()

	// Create multiple YAML files
	files := map[string]string{
		"agents1.yaml": `
version: "1.0"
agents:
  - name: agent-one
    display_name: "Agent One"
    description: "First agent"
    category: "test"
`,
		"agents2.yaml": `
version: "1.0"
agents:
  - name: agent-two
    display_name: "Agent Two"
    description: "Second agent"
    category: "test"
`,
		"readme.md": "# Rules\nThis is not a yaml file.",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	loader := NewLoader()
	rf, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error: %v", err)
	}

	if len(rf.Agents) != 2 {
		t.Fatalf("len(Agents) = %d, want 2", len(rf.Agents))
	}
}

func TestLoader_Parse_InvalidYAML(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Parse([]byte(`:invalid: yaml: [`))
	if err == nil {
		t.Error("Parse() should return error for invalid YAML")
	}
}
