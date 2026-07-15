package skill

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/model"
)

// SKILL.md is the only high-quality skill signal per the Agent Skills
// specification (https://agentskills.io/specification).
const skillFileName = "SKILL.md"

// Discoverer finds and parses SKILL.md files.
type Discoverer struct{}

// NewDiscoverer creates a new Discoverer.
func NewDiscoverer() *Discoverer {
	return &Discoverer{}
}

// DiscoverSkills discovers skills for a set of rules.
// It first scans explicit scan_paths, then (if auto_discover is true)
// probes common subdirectory names under the agent's file-evidence directories.
func (d *Discoverer) DiscoverSkills(rule model.AgentRule) ([]model.Skill, error) {
	if rule.Skills == nil {
		return nil, nil
	}

	sr := rule.Skills
	var allSkills []model.Skill

	// Phase 1: explicit scan paths from the rule
	paths, err := config.ResolveSkillPaths(sr.ScanPaths)
	if err != nil {
		return nil, err
	}

	for _, scanPath := range paths {
		skills := d.scanPath(scanPath, sr)
		allSkills = append(allSkills, skills...)
	}

	return allSkills, nil
}

// Common skill subdirectory names to probe under agent home directories.
var skillProbeNames = []string{
	"skills", "agents", "tools", "instructions", "prompts",
	"rules", "commands", "workflows", ".skills", ".agent",
}

// ProbeSkillDirs searches for skill directories by probing well-known
// subdirectory names under the given file-evidence directories.
// Returns the list of resolved existing directories.
func ProbeSkillDirs(fileDirs []string) []string {
	var found []string
	for _, dir := range fileDirs {
		for _, name := range skillProbeNames {
			probePath := filepath.Join(dir, name)
			if info, err := os.Stat(probePath); err == nil && info.IsDir() {
				found = append(found, probePath)
			}
		}
	}
	return found
}

// DiscoverSkillsWithProbe discovers skills using both explicit paths and
// auto-probed directories derived from file-evidence.
// skillDirOut receives the first successfully scanned directory path.
func (d *Discoverer) DiscoverSkillsWithProbe(rule model.AgentRule, fileDirs []string, skillDirOut *string) ([]model.Skill, error) {
	sr := rule.Skills
	if sr == nil {
		return nil, nil
	}

	var allSkills []model.Skill
	var firstDir string

	// Phase 1: explicit scan paths from the rule
	paths, err := config.ResolveSkillPaths(sr.ScanPaths)
	if err != nil {
		return nil, err
	}
	for _, scanPath := range paths {
		if firstDir == "" {
			firstDir = scanPath
		}
		skills := d.scanPath(scanPath, sr)
		allSkills = append(allSkills, skills...)
	}

	// Phase 2: auto-probe if enabled (defaults to true when skills.enabled)
	if (sr.AutoDiscover == nil || *sr.AutoDiscover) && len(fileDirs) > 0 {
		probed := ProbeSkillDirs(fileDirs)
		for _, probePath := range probed {
			// Skip already-scanned paths (avoid scanning same dir twice)
			if isInPaths(probePath, paths) {
				continue
			}
			if firstDir == "" {
				firstDir = probePath
			}
			skills := d.scanPath(probePath, sr)
			allSkills = append(allSkills, skills...)
		}
	}

	if skillDirOut != nil && firstDir != "" {
		*skillDirOut = firstDir
	}

	return allSkills, nil
}

func isInPaths(target string, paths []string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

// GlobalSkillRule returns a SkillRule with default scanning parameters
// suitable for a global walk.
func GlobalSkillRule() *model.SkillRule {
	return &model.SkillRule{
		MaxDepth:  3,
		MaxSizeKB: 100,
		MinSizeKB: 1,
	}
}

// ScanPath walks a directory tree and parses every SKILL.md file it finds.
// This is the public version usable for global walks.
func (d *Discoverer) ScanPath(root string, sr *model.SkillRule) []model.Skill {
	return d.scanPath(root, sr)
}

// scanPath walks a directory tree and parses every SKILL.md file it finds.
// Only files named exactly "SKILL.md" (case-insensitive) are considered.
func (d *Discoverer) scanPath(root string, sr *model.SkillRule) []model.Skill {
	var skills []model.Skill

	maxDepth := sr.MaxDepth
	if maxDepth == 0 {
		maxDepth = 3
	}
	maxSize := sr.MaxSizeKB * 1024
	if maxSize == 0 {
		maxSize = 102400 // 100KB default
	}
	minSize := sr.MinSizeKB * 1024
	if minSize == 0 {
		minSize = 1024 // 1KB default
	}

	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Calculate depth
		rel, _ := filepath.Rel(root, path)
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if depth > maxDepth && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}

		// Only match files named SKILL.md (case-insensitive)
		if !strings.EqualFold(entry.Name(), skillFileName) {
			return nil
		}

		// Size filter
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.Size() < int64(minSize) {
			return nil
		}
		if info.Size() > int64(maxSize) {
			return nil
		}

		// Parse the SKILL.md file
		skill, err := d.parseSkillFile(path)
		if err != nil {
			return nil
		}
		if skill == nil {
			return nil
		}

		skills = append(skills, *skill)
		return nil
	})

	return skills
}

// parseSkillFile parses a single SKILL.md file.
func (d *Discoverer) parseSkillFile(path string) (*model.Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Normalize line endings: \r\n -> \n (Windows files may have CRLF)
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return d.parseSKILLmd(path, content)
}

// parseSKILLmd parses a SKILL.md file per the Agent Skills specification:
// https://agentskills.io/specification
//
// SKILL.md has YAML frontmatter between --- delimiters with fields:
// name, description, license, compatibility, metadata, allowed-tools.
// If name is not in frontmatter, the parent directory name is used.
func (d *Discoverer) parseSKILLmd(path, content string) (*model.Skill, error) {
	skill := &model.Skill{
		FilePath: path,
		Format:   "markdown",
	}

	// Parse YAML frontmatter (between --- delimiters)
	if strings.HasPrefix(content, "---\n") {
		end := findFrontmatterEnd(content)
		if end > 0 {
			fm := content[4:end]
			var fmData map[string]any
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				d.extractFromMap(skill, fmData)
				// Extract Agent Skills spec fields
				d.extractSkillsSpecFields(skill, fmData)
			}
		}
	}

	// If no frontmatter, parse by markdown sections
	if skill.Name == "" {
		d.parseMarkdownSections(skill, content)
	}

	// Per Agent Skills spec: if name is still empty, use the parent
	// directory name (the skill directory name)
	if skill.Name == "" {
		dirName := filepath.Base(filepath.Dir(path))
		if dirName != "." && dirName != "/" && dirName != "" {
			skill.Name = dirName
		}
	}

	// Trim whitespace and carriage returns from name (Windows \r\n issue)
	skill.Name = strings.TrimSpace(skill.Name)
	skill.Description = strings.TrimSpace(skill.Description)
	skill.DisplayName = strings.TrimSpace(skill.DisplayName)

	if skill.Name == "" {
		return nil, nil
	}

	return skill, nil
}

// extractSkillsSpecFields extracts fields defined by the Agent Skills
// specification from YAML frontmatter.
func (d *Discoverer) extractSkillsSpecFields(skill *model.Skill, fm map[string]any) {
	// license — SPDX identifier or "proprietary"
	if v, ok := fm["license"].(string); ok {
		skill.License = v
	}

	// compatibility — version range string, e.g. ">=1.0.0"
	if v, ok := fm["compatibility"].(string); ok {
		skill.Compatibility = v
	}

	// allowed-tools → tools array
	if tools, ok := fm["allowed-tools"].([]any); ok {
		for _, t := range tools {
			switch tool := t.(type) {
			case string:
				skill.Tools = append(skill.Tools, model.SkillTool{Name: tool})
			case map[string]any:
				st := model.SkillTool{}
				if n, ok := tool["name"].(string); ok {
					st.Name = n
				}
				if desc, ok := tool["description"].(string); ok {
					st.Description = desc
				}
				skill.Tools = append(skill.Tools, st)
			}
		}
	}

	// metadata — extract version if present (spec says version can live
	// inside metadata)
	if meta, ok := fm["metadata"].(map[string]any); ok {
		if skill.Metadata == nil {
			skill.Metadata = meta
		} else {
			for k, v := range meta {
				skill.Metadata[k] = v
			}
		}
		// Version from metadata takes precedence if not already set
		if skill.Version == "" {
			if v, ok := meta["version"].(string); ok {
				skill.Version = v
			}
		}
	}
}

func findFrontmatterEnd(content string) int {
	// Find the second ---
	idx := strings.Index(content[4:], "\n---\n")
	if idx < 0 {
		return -1
	}
	return idx + 4
}

func (d *Discoverer) parseMarkdownSections(skill *model.Skill, content string) {
	lines := strings.Split(content, "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") && skill.Name == "" {
			skill.Name = strings.TrimPrefix(line, "# ")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			d.applySection(skill, currentSection, sectionContent.String())
			currentSection = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			sectionContent.Reset()
			continue
		}
		sectionContent.WriteString(line)
		sectionContent.WriteString("\n")
	}
	// Last section
	d.applySection(skill, currentSection, sectionContent.String())
}

func (d *Discoverer) applySection(skill *model.Skill, section, content string) {
	trimmed := strings.TrimSpace(content)
	switch {
	case strings.Contains(section, "description") || strings.Contains(section, "简介"):
		skill.Description = trimmed
	case strings.Contains(section, "tool"):
		d.parseToolList(skill, trimmed)
	case strings.Contains(section, "prompt") || strings.Contains(section, "instruction"):
		skill.PromptTemplate = trimmed
	case strings.Contains(section, "parameter"):
		d.parseParameterSection(skill, trimmed)
	}
}

func (d *Discoverer) parseToolList(skill *model.Skill, content string) {
	// Try to parse as bullet list of tools
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			toolName := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			toolName = strings.TrimSpace(toolName)
			if toolName != "" {
				skill.Tools = append(skill.Tools, model.SkillTool{Name: toolName})
			}
		}
	}
}

func (d *Discoverer) parseParameterSection(skill *model.Skill, content string) {
	if skill.Parameters == nil {
		skill.Parameters = make(map[string]any)
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			skill.Parameters[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
}

func (d *Discoverer) extractFromMap(skill *model.Skill, data map[string]any) {
	if v, ok := data["name"].(string); ok {
		skill.Name = strings.TrimSpace(v)
	}
	if v, ok := data["display_name"].(string); ok {
		skill.DisplayName = strings.TrimSpace(v)
	}
	if v, ok := data["description"].(string); ok {
		skill.Description = strings.TrimSpace(v)
	}
	if v, ok := data["version"].(string); ok {
		skill.Version = strings.TrimSpace(v)
	}
	if v, ok := data["prompt"].(string); ok {
		skill.PromptTemplate = strings.TrimSpace(v)
	}
	if v, ok := data["prompt_template"].(string); ok {
		skill.PromptTemplate = strings.TrimSpace(v)
	}
	if tools, ok := data["tools"].([]any); ok {
		for _, t := range tools {
			switch tool := t.(type) {
			case string:
				skill.Tools = append(skill.Tools, model.SkillTool{Name: tool})
			case map[string]any:
				st := model.SkillTool{}
				if n, ok := tool["name"].(string); ok {
					st.Name = n
				}
				if desc, ok := tool["description"].(string); ok {
					st.Description = desc
				}
				if params, ok := tool["parameters"]; ok {
					st.Parameters, _ = params.(map[string]any)
				}
				skill.Tools = append(skill.Tools, st)
			}
		}
	}
	if params, ok := data["parameters"].(map[string]any); ok {
		skill.Parameters = params
	}
	if triggers, ok := data["trigger_patterns"].([]any); ok {
		for _, t := range triggers {
			if s, ok := t.(string); ok {
				skill.TriggerPatterns = append(skill.TriggerPatterns, s)
			}
		}
	}
	if deps, ok := data["dependencies"].([]any); ok {
		for _, d := range deps {
			if s, ok := d.(string); ok {
				skill.Dependencies = append(skill.Dependencies, s)
			}
		}
	}
	if meta, ok := data["metadata"].(map[string]any); ok {
		skill.Metadata = meta
	}
}
