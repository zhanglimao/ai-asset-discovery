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
	skillPath := filepath.Join(dir, skillFileName)
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

	skill, err := d.parseSkillFile(skillPath)
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
	skillPath := filepath.Join(dir, skillFileName)
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

	skill, err := d.parseSkillFile(skillPath)
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

func TestDiscoverer_ScanPath(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	os.MkdirAll(skillDir, 0755)

	// SKILL.md with valid frontmatter — should be discovered
	os.WriteFile(filepath.Join(skillDir, skillFileName),
		[]byte("---\nname: code-review\ndescription: Review code\n---\n# Review\n"+
			strings.Repeat("This is a skill for reviewing code changes.\n", 40)), 0644)

	// Nested SKILL.md
	nestedDir := filepath.Join(skillDir, "deploy")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, skillFileName),
		[]byte("---\nname: deploy\ndescription: Deploy apps\n---\n"+
			strings.Repeat("# Deploy skill\n", 40)), 0644)

	// Random .yaml file — should be IGNORED (not SKILL.md)
	os.WriteFile(filepath.Join(skillDir, "random-config.yaml"),
		[]byte("name: not-a-skill\ndescription: Should be ignored\n"), 0644)

	// README.md — should be IGNORED (not SKILL.md)
	os.WriteFile(filepath.Join(skillDir, "README.md"),
		[]byte("# Skills\nThis is documentation"), 0644)

	sr := &model.SkillRule{
		MaxDepth:  3,
		MaxSizeKB: 100,
	}

	skills := d.scanPath(skillDir, sr)
	if len(skills) < 2 {
		t.Errorf("len(skills) = %d, want at least 2 (code-review + deploy SKILL.md)", len(skills))
	}
}

func TestDiscoverer_ScanPath_MaxDepth(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	deepDir := filepath.Join(dir, "level1", "level2", "level3", "level4")
	os.MkdirAll(deepDir, 0755)
	os.WriteFile(filepath.Join(deepDir, skillFileName),
		[]byte("---\nname: deep\ndescription: Too deep\n---\n"+strings.Repeat("x", 40)), 0644)

	sr := &model.SkillRule{
		MaxDepth:  2,
		MaxSizeKB: 100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills at shallow depth, got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_NonSkillMdIgnored(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	// Files that are NOT named SKILL.md should be completely ignored
	os.WriteFile(filepath.Join(dir, "deploy-skill.md"),
		[]byte("---\nname: deploy\ndescription: Deploy apps\n---\n"+strings.Repeat("x", 100)), 0644)
	os.WriteFile(filepath.Join(dir, "my-skill.md"),
		[]byte("---\nname: my-skill\ndescription: My Skill\n---\n"+strings.Repeat("x", 100)), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("name: config-skill\ndescription: Config\n"), 0644)

	sr := &model.SkillRule{
		MaxDepth:  3,
		MaxSizeKB: 100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (only SKILL.md matched, not generic .md/.yaml), got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_CaseInsensitiveSkILLmd(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	// Create skill.md (lowercase) — should match via EqualFold
	os.WriteFile(filepath.Join(dir, "skill.md"),
		[]byte("---\nname: case-test\ndescription: Case insensitive\n---\n"+strings.Repeat("x", 100)), 0644)

	sr := &model.SkillRule{
		MaxDepth:  3,
		MaxSizeKB: 100,
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 1 {
		t.Errorf("expected 1 skill (case-insensitive SKILL.md match), got %d", len(skills))
	}
}

func TestDiscoverer_ScanPath_SizeLimits(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()

	// File too large (>MaxSize)
	bigContent := strings.Repeat("x", 200000) // 200KB
	os.WriteFile(filepath.Join(dir, skillFileName), []byte(bigContent), 0644)

	sr := &model.SkillRule{
		MaxDepth:  3,
		MaxSizeKB: 100, // 100KB max
	}

	skills := d.scanPath(dir, sr)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (oversized SKILL.md), got %d", len(skills))
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

func TestDiscoverer_ParseSkillFile_InvalidMarkdown(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, skillFileName)
	// SKILL.md with no name at all — should still parse with fallback to dir name
	os.WriteFile(skillPath, []byte("Just some text with no structure"), 0644)

	skill, err := d.parseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("parseSkillFile() unexpected error: %v", err)
	}
	if skill == nil {
		t.Fatal("expected non-nil skill even for content with no extractable name (fallback to dir name)")
	}
	if skill.Name == "" {
		t.Error("expected non-empty Name via fallback, got empty")
	}
}

func TestDiscoverer_ParseMarkdown_SectionsBasedParsing(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, skillFileName)
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

	skill, err := d.parseSkillFile(skillPath)
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

func TestDiscoverer_ParseYAMLFrontmatter_ComplexTools(t *testing.T) {
	d := NewDiscoverer()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, skillFileName)
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

	skill, err := d.parseSkillFile(skillPath)
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

// ============================================================
// auto_discover tests
// ============================================================

// setupSkillDir creates a temp file-evidence directory with a populated
// "skills" subdirectory containing a valid SKILL.md file.
func setupSkillDir(t *testing.T) (fileDir string) {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, skillFileName),
		[]byte("---\nname: test-skill\ndescription: Auto-probed test skill\n---\n"+
			strings.Repeat("This is a skill discovered via auto-probe.\n", 40)), 0644)
	return dir
}

func TestDiscoverSkillsWithProbe_AutoDiscoverNilDefaultsToTrue(t *testing.T) {
	d := NewDiscoverer()
	fileDir := setupSkillDir(t)

	autoDiscover := (*bool)(nil) // omitted in YAML → nil in Go
	sr := &model.SkillRule{
		Enabled:      true,
		AutoDiscover: autoDiscover,
	}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, []string{fileDir}, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if len(skills) == 0 {
		t.Error("auto_discover=nil should default to true; expected skills from auto-probe, got none")
	}
	if skillDirOut == "" {
		t.Error("auto_discover=nil should set skillDirOut")
	}
}

func TestDiscoverSkillsWithProbe_AutoDiscoverExplicitTrue(t *testing.T) {
	d := NewDiscoverer()
	fileDir := setupSkillDir(t)

	autoDiscover := true
	sr := &model.SkillRule{
		Enabled:      true,
		AutoDiscover: &autoDiscover,
	}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, []string{fileDir}, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if len(skills) == 0 {
		t.Error("auto_discover=true should probe; expected skills, got none")
	}
}

func TestDiscoverSkillsWithProbe_AutoDiscoverExplicitFalse(t *testing.T) {
	d := NewDiscoverer()
	fileDir := setupSkillDir(t)

	autoDiscover := false
	sr := &model.SkillRule{
		Enabled:      true,
		AutoDiscover: &autoDiscover,
	}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, []string{fileDir}, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("auto_discover=false should NOT probe; expected 0 skills, got %d", len(skills))
	}
}

func TestDiscoverSkillsWithProbe_NoFileDirsSkipsProbe(t *testing.T) {
	d := NewDiscoverer()
	_ = setupSkillDir(t) // create dir but don't pass to function

	sr := &model.SkillRule{
		Enabled: true,
	}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	// empty fileDirs — auto-probe should be skipped even with default=true
	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, nil, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("empty fileDirs should skip auto-probe; expected 0 skills, got %d", len(skills))
	}
}

func TestDiscoverSkillsWithProbe_NilSkills(t *testing.T) {
	d := NewDiscoverer()
	rule := model.AgentRule{Name: "no-skills", Skills: nil}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, nil, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if skills != nil {
		t.Errorf("nil Skills should return nil, got %v", skills)
	}
}

func TestDiscoverSkillsWithProbe_ExplicitPathsPlusProbe(t *testing.T) {
	d := NewDiscoverer()
	fileDir := setupSkillDir(t)

	// Create an explicit path with its own SKILL.md
	explicitDir := filepath.Join(t.TempDir(), "explicit-skills")
	os.MkdirAll(explicitDir, 0755)
	os.WriteFile(filepath.Join(explicitDir, skillFileName),
		[]byte("---\nname: explicit-skill\ndescription: Explicit scan_paths test skill\n---\n"+
			strings.Repeat("# explicit skill\n", 30)), 0644)

	sr := &model.SkillRule{
		Enabled:   true,
		ScanPaths: []string{explicitDir},
	}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, []string{fileDir}, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	// Should find: 1 from explicit path + 1 from auto-probe
	if len(skills) < 2 {
		t.Errorf("expected at least 2 skills (1 explicit + 1 auto-probed), got %d", len(skills))
	}
}

// TestDiscoverSkillsWithProbe_SKILLmdIsTheOnlyMatch verifies that
// only files named SKILL.md (case-insensitive) are discovered.
func TestDiscoverSkillsWithProbe_SKILLmdIsTheOnlyMatch(t *testing.T) {
	d := NewDiscoverer()
	fileDir := t.TempDir()
	skillDir := filepath.Join(fileDir, "skills")
	os.MkdirAll(skillDir, 0755)

	// SKILL.md — should be found
	os.WriteFile(filepath.Join(skillDir, skillFileName),
		[]byte("---\nname: real-skill\ndescription: Real SKILL.md\n---\n"+
			strings.Repeat("pad", 30)), 0644)
	// skill.md (lowercase) — should also be found (case-insensitive)
	os.WriteFile(filepath.Join(skillDir, "skill.md"),
		[]byte("---\nname: lower-skill\ndescription: Lowercase skill.md\n---\n"+
			strings.Repeat("pad", 30)), 0644)
	// random.md — should be ignored
	os.WriteFile(filepath.Join(skillDir, "random.md"),
		[]byte("---\nname: random\ndescription: Not SKILL.md\n---\n"+
			strings.Repeat("pad", 30)), 0644)

	sr := &model.SkillRule{Enabled: true}
	rule := model.AgentRule{Name: "test-agent", Skills: sr}

	var skillDirOut string
	skills, err := d.DiscoverSkillsWithProbe(rule, []string{fileDir}, &skillDirOut)
	if err != nil {
		t.Fatalf("DiscoverSkillsWithProbe() error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected exactly 2 skills (SKILL.md + skill.md, not random.md), got %d", len(skills))
	}
}

func TestProbeSkillDirs_FindsCommonNames(t *testing.T) {
	dir := t.TempDir()
	expected := map[string]bool{}
	for _, name := range skillProbeNames {
		p := filepath.Join(dir, name)
		os.MkdirAll(p, 0755)
		expected[p] = true
	}

	found := ProbeSkillDirs([]string{dir})
	if len(found) != len(skillProbeNames) {
		t.Errorf("ProbeSkillDirs: expected %d dirs, got %d", len(skillProbeNames), len(found))
	}
	for _, p := range found {
		if !expected[p] {
			t.Errorf("unexpected probed dir: %s", p)
		}
	}
}

func TestProbeSkillDirs_SkipsNonExistent(t *testing.T) {
	dir := t.TempDir()
	found := ProbeSkillDirs([]string{dir})
	if len(found) != 0 {
		t.Errorf("ProbeSkillDirs on empty dir: expected 0, got %d", len(found))
	}
}

func TestProbeSkillDirs_MultipleBaseDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.MkdirAll(filepath.Join(dir1, "skills"), 0755)
	os.MkdirAll(filepath.Join(dir2, "tools"), 0755)

	found := ProbeSkillDirs([]string{dir1, dir2})
	if len(found) != 2 {
		t.Errorf("ProbeSkillDirs on 2 base dirs: expected 2, got %d: %v", len(found), found)
	}
}
