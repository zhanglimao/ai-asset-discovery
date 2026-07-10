package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-asset-discovery/internal/model"
)

func TestDiscoverer_ParseMarkdownFrontmatter(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "code-review.md")
	content := `---
name: code-review
description: Review code for issues
version: "1.0"
tools:
  - name: read_file
    description: Read file contents
  - grep
parameters:
  language: python
  severity: high
prompt: "You are a code reviewer..."
trigger_patterns:
  - "review.*code"
dependencies:
  - "git-diff"
---
# Code Review Skill

## Description
This skill reviews code for issues.
`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".md")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name != "code-review" {
		t.Errorf("Name = %q, want %q", skill.Name, "code-review")
	}
	if skill.Description != "Review code for issues" {
		t.Errorf("Description = %q, want %q", skill.Description, "Review code for issues")
	}
	if skill.Version != "1.0" {
		t.Errorf("Version = %q, want %q", skill.Version, "1.0")
	}
	if skill.PromptTemplate != "You are a code reviewer..." {
		t.Errorf("PromptTemplate = %q", skill.PromptTemplate)
	}
	if skill.Format != "markdown" {
		t.Errorf("Format = %q, want markdown", skill.Format)
	}
	if len(skill.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(skill.Tools))
	}
	if skill.Tools[0].Name != "read_file" {
		t.Errorf("Tools[0].Name = %q, want read_file", skill.Tools[0].Name)
	}
	if skill.Tools[0].Description != "Read file contents" {
		t.Errorf("Tools[0].Description = %q", skill.Tools[0].Description)
	}
	if skill.Tools[1].Name != "grep" {
		t.Errorf("Tools[1].Name = %q, want grep", skill.Tools[1].Name)
	}
	if len(skill.TriggerPatterns) != 1 {
		t.Errorf("len(TriggerPatterns) = %d, want 1", len(skill.TriggerPatterns))
	}
	if len(skill.Dependencies) != 1 {
		t.Errorf("len(Dependencies) = %d, want 1", len(skill.Dependencies))
	}
	if skill.Parameters == nil {
		t.Error("Parameters is nil")
	}
}

func TestDiscoverer_ParseMarkdownSections(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "refactor.md")
	content := `# Refactoring Skill

## Description
Automated code refactoring.

## Tools
- rename_symbol
- find_references
- move_to_file

## Parameters
language: go
scope: workspace

## Instructions
When asked to refactor, follow these steps...
`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".md")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name != "Refactoring Skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "Refactoring Skill")
	}
	if skill.Description != "Automated code refactoring." {
		t.Errorf("Description = %q", skill.Description)
	}
	if len(skill.Tools) != 3 {
		t.Fatalf("len(Tools) = %d, want 3", len(skill.Tools))
	}
}

func TestDiscoverer_ParseYAMLSkill(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "deploy.yaml")
	content := `
name: deploy-app
description: Deploy the application to production
version: "2.1.0"
tools:
  - name: run_command
    description: Execute shell commands
  - name: check_status
parameters:
  environment:
    type: string
    enum: [staging, production]
prompt: "You are a deployment specialist..."
`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".yaml")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name != "deploy-app" {
		t.Errorf("Name = %q, want deploy-app", skill.Name)
	}
	if skill.Format != "yaml" {
		t.Errorf("Format = %q, want yaml", skill.Format)
	}
}

func TestDiscoverer_ParseJSONSkill(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "test.json")
	content := `{"name": "test-runner", "description": "Run tests automatically", "version": "1.0"}`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".json")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}

	if skill.Name != "test-runner" {
		t.Errorf("Name = %q", skill.Name)
	}
	if skill.Format != "json" {
		t.Errorf("Format = %q, want json", skill.Format)
	}
}

func TestDiscoverer_ContainsAnyKeyword(t *testing.T) {
	d := NewDiscoverer()

	skill := &model.Skill{
		Name:        "code-review",
		Description: "Review code for bugs and style issues",
	}

	tests := []struct {
		name     string
		keywords []string
		want     bool
	}{
		{"empty keywords", nil, true},
		{"matching keyword", []string{"review", "test"}, true},
		{"matching description", []string{"bugs"}, true},
		{"no match", []string{"deploy", "monitor"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.containsAnyKeyword(skill, tt.keywords)
			if got != tt.want {
				t.Errorf("containsAnyKeyword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoverer_ScanPath(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()

	// Create some skill files
	skillDir := filepath.Join(dir, "skills")
	os.MkdirAll(skillDir, 0755)

	os.WriteFile(filepath.Join(skillDir, "code-review.md"),
		[]byte("---\nname: code-review\ndescription: Review code\n---\n# Review\nThis is a skill for reviewing code changes in detail.\nIt has enough content to exceed the minimum 1KB size filter for the full content check.\n"+strings.Repeat("pad", 500)), 0644)
	os.WriteFile(filepath.Join(skillDir, "deploy.yaml"),
		[]byte("name: deploy\ndescription: Deploy apps\n# "+strings.Repeat("pad", 500)), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"),
		[]byte("# Skills\nThis is documentation"), 0644) // No name + README exclusion, should be skipped
	os.WriteFile(filepath.Join(skillDir, "small.txt"),
		[]byte("tiny"), 0644) // Too small, should be skipped by minSize

	sr := &model.SkillRule{
		Extensions: []string{".md", ".yaml", ".yml", ".json", ".txt"},
		Keywords:   []string{"code", "deploy"},
		MaxDepth:   3,
		MaxSizeKB:  100,
	}

	// The scanPath now has 1KB min size filter and keyword pre-check.
	// With the padded files above, code-review.md and deploy.yaml should both pass.
	skills := d.scanPath(skillDir, sr)
	if len(skills) < 2 {
		t.Errorf("len(skills) = %d, want at least 2", len(skills))
	}
}

func TestDiscoverer_IsAllowedExt(t *testing.T) {
	d := NewDiscoverer()

	allowed := []string{".md", ".yaml", ".json"}

	if !d.isAllowedExt(".md", allowed) {
		t.Error(".md should be allowed")
	}
	if !d.isAllowedExt(".MD", allowed) {
		t.Error(".MD should be allowed (case insensitive)")
	}
	if d.isAllowedExt(".txt", allowed) {
		t.Error(".txt should not be allowed")
	}
}

func TestDiscoverer_ScanPath_MaxDepth(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	// Create nested directories
	deepDir := filepath.Join(dir, "level1", "level2", "level3", "level4")
	os.MkdirAll(deepDir, 0755)

	sr := &model.SkillRule{
		Extensions: []string{".md"},
		MaxDepth:   2,
		MaxSizeKB:  100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills at shallow depth, got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_SkipHidden(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".hidden-skill.md"),
		[]byte("---\nname: hidden\ndescription: Hidden\n---\n"+strings.Repeat("x", 1024)), 0644)

	sr := &model.SkillRule{
		Extensions: []string{".md"},
		MaxDepth:   3,
		MaxSizeKB:  100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (hidden file), got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_SkipTemplateFiles(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "todo-skill.md"),
		[]byte("---\nname: todo\ndescription: TODO\n---\n"+strings.Repeat("x", 1024)), 0644)
	os.WriteFile(filepath.Join(dir, "example-skill.md"),
		[]byte("---\nname: example\ndescription: Example\n---\n"+strings.Repeat("x", 1024)), 0644)

	sr := &model.SkillRule{
		Extensions: []string{".md"},
		MaxDepth:   3,
		MaxSizeKB:  100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (template files), got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_KeywordFilter(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "deploy-skill.md"),
		[]byte("---\nname: deploy\ndescription: Deploy apps\n---\n"+strings.Repeat("x", 1024)), 0644)

	sr := &model.SkillRule{
		Extensions: []string{".md"},
		MaxDepth:   3,
		MaxSizeKB:  100,
		Keywords:   []string{"code-review"}, // won't match deploy
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (keyword mismatch), got %d", len(skills))
	}
}

func TestDiscoverer_DiscoverSkills_NoRules(t *testing.T) {
	d := NewDiscoverer()
	rule := model.AgentRule{Name: "no-skills"}
	skills, err := d.DiscoverSkills(rule)
	if err != nil {
		t.Fatalf("DiscoverSkills() error: %v", err)
	}
	if skills != nil {
		t.Errorf("expected nil skills, got %v", skills)
	}
}

func TestDiscoverer_ParseSkillFile_InvalidJSON(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "bad.json")
	os.WriteFile(skillPath, []byte("{invalid json"), 0644)

	_, err := d.parseSkillFile(skillPath, ".json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDiscoverer_ParseSkillFile_UnsupportedFormat(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "notes.txt")
	os.WriteFile(skillPath, []byte("some text"), 0644)

	skill, err := d.parseSkillFile(skillPath, ".txt")
	// Unsupported formats return nil, nil (not an error)
	if err != nil {
		t.Fatalf("parseSkillFile() unexpected error: %v", err)
	}
	if skill != nil {
		t.Errorf("expected nil skill for unsupported format, got %+v", skill)
	}
}

func TestDiscoverer_ParseMarkdown_SectionsBasedParsing(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "test.md")
	// No frontmatter, only sections
	content := `# Test Skill

## Description
A test skill

## Instructions
Follow these steps...

## Examples
- Example 1
- Example 2
`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".md")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}
	if skill.Name != "Test Skill" {
		t.Errorf("Name = %q, want Test Skill", skill.Name)
	}
}

func TestDiscoverer_ContainsAnyKeyword_EdgeCases(t *testing.T) {
	d := NewDiscoverer()

	skill := &model.Skill{
		Name:        "",
		Description: "",
	}

	// Empty skill with non-empty keywords should return false
	if d.containsAnyKeyword(skill, []string{"test"}) {
		t.Error("expected false for skill with empty name/description")
	}

	// Empty keywords should return true
	if !d.containsAnyKeyword(skill, nil) {
		t.Error("expected true for nil keywords")
	}
}

func TestDiscoverer_ScanPath_SizeLimits(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()

	// File too small (<1KB)
	os.WriteFile(filepath.Join(dir, "tiny.md"), []byte("tiny"), 0644)

	// File too large (>MaxSize)
	bigContent := strings.Repeat("x", 200000) // 200KB
	os.WriteFile(filepath.Join(dir, "big.md"), []byte(bigContent), 0644)

	sr := &model.SkillRule{
		Extensions: []string{".md"},
		MaxDepth:   3,
		MaxSizeKB:  100, // 100KB max
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (size limits), got %d", len(skills))
	}
}

func TestDiscoverer_ParseYAMLFrontmatter_ComplexTools(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "complex.md")
	content := `---
name: advanced-skill
description: Advanced skill with complex metadata
version: "3.0"
tools:
  - name: tool_a
    description: First tool
  - tool_b
  - name: tool_c
dependencies:
  - dep_a
  - dep_b
trigger_patterns:
  - pattern1
  - pattern2
  - pattern3
---
# Advanced Skill
`
	os.WriteFile(skillPath, []byte(content), 0644)

	skill, err := d.parseSkillFile(skillPath, ".md")
	if err != nil {
		t.Fatalf("parseSkillFile() error: %v", err)
	}
	if len(skill.Tools) != 3 {
		t.Errorf("len(Tools) = %d, want 3", len(skill.Tools))
	}
	if len(skill.Dependencies) != 2 {
		t.Errorf("len(Dependencies) = %d, want 2", len(skill.Dependencies))
	}
	if len(skill.TriggerPatterns) != 3 {
		t.Errorf("len(TriggerPatterns) = %d, want 3", len(skill.TriggerPatterns))
	}
}
