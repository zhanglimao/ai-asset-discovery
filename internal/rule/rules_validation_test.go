package rule

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/model"
)

// loadAgentsYAML loads and parses the actual agents.yaml rules file.
func loadAgentsYAML(t *testing.T) *model.RulesFile {
	t.Helper()

	// Resolve rules/agents.yaml relative to project root (go up 2 levels from internal/rule)
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	rulesPath := filepath.Join(projectRoot, "rules", "agents.yaml")

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read agents.yaml: %v", err)
	}

	loader := NewLoader()
	rf, err := loader.Parse(data)
	if err != nil {
		t.Fatalf("parse agents.yaml: %v", err)
	}
	return rf
}

func findProjectRoot() (string, error) {
	// Walk up from the test file's directory looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// TestAgentsYAML_AllRulesParse validates that every rule in agents.yaml parses correctly.
func TestAgentsYAML_AllRulesParse(t *testing.T) {
	rf := loadAgentsYAML(t)

	if rf.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", rf.Version)
	}
	if len(rf.Agents) == 0 {
		t.Fatal("no agents found in rules file")
	}

	t.Logf("Loaded %d agent rules from agents.yaml", len(rf.Agents))

	for _, agent := range rf.Agents {
		if agent.Name == "" {
			t.Error("found agent rule with empty name")
		}
	}
}

// TestAgentsYAML_MinConfidenceValues validates that all min_confidence values are valid.
func TestAgentsYAML_MinConfidenceValues(t *testing.T) {
	rf := loadAgentsYAML(t)
	validConfidences := map[string]bool{"confirmed": true, "possible": true, "ghost": true}

	for _, agent := range rf.Agents {
		if !validConfidences[agent.MinConfidence] {
			t.Errorf("agent %q has invalid min_confidence %q (want: confirmed, possible, or ghost)",
				agent.Name, agent.MinConfidence)
		}
	}
}

// TestAgentsYAML_ProcessRules validates process detection rules consistency.
func TestAgentsYAML_ProcessRules(t *testing.T) {
	rf := loadAgentsYAML(t)
	validTypes := map[string]bool{"exact": true, "contains": true, "regex": true}
	validMatchLogic := map[string]bool{"and": true, "or": true, "": true} // empty defaults to "or"

	for _, agent := range rf.Agents {
		if agent.Process == nil {
			continue
		}
		pr := agent.Process

		// match_logic must be valid
		if !validMatchLogic[pr.MatchLogic] {
			t.Errorf("agent %q: invalid process.match_logic %q", agent.Name, pr.MatchLogic)
		}

		// Must have at least one pattern field
		hasPatterns := len(pr.NamePatterns) > 0 || len(pr.CmdPatterns) > 0 ||
			len(pr.ExePatterns) > 0 || len(pr.DirPatterns) > 0
		if !hasPatterns {
			t.Errorf("agent %q: process rule has no pattern fields", agent.Name)
		}

		// Validate name_patterns
		for i, p := range pr.NamePatterns {
			if !validTypes[p.Type] {
				t.Errorf("agent %q: name_patterns[%d] invalid type %q", agent.Name, i, p.Type)
			}
			if p.Value == "" {
				t.Errorf("agent %q: name_patterns[%d] empty value", agent.Name, i)
			}
			if p.Type == "regex" {
				if err := validateRegex(p.Value); err != nil {
					t.Errorf("agent %q: name_patterns[%d] regex %q: %v", agent.Name, i, p.Value, err)
				}
			}
		}

		// Validate cmd_patterns
		for i, p := range pr.CmdPatterns {
			if !validTypes[p.Type] {
				t.Errorf("agent %q: cmd_patterns[%d] invalid type %q", agent.Name, i, p.Type)
			}
			if p.Value == "" {
				t.Errorf("agent %q: cmd_patterns[%d] empty value", agent.Name, i)
			}
			if p.Type == "regex" {
				if err := validateRegex(p.Value); err != nil {
					t.Errorf("agent %q: cmd_patterns[%d] regex %q: %v", agent.Name, i, p.Value, err)
				}
			}
		}

		// Validate exe_patterns
		for i, p := range pr.ExePatterns {
			if !validTypes[p.Type] {
				t.Errorf("agent %q: exe_patterns[%d] invalid type %q", agent.Name, i, p.Type)
			}
			if p.Value == "" {
				t.Errorf("agent %q: exe_patterns[%d] empty value", agent.Name, i)
			}
			if p.Type == "regex" {
				if err := validateRegex(p.Value); err != nil {
					t.Errorf("agent %q: exe_patterns[%d] regex %q: %v", agent.Name, i, p.Value, err)
				}
			}
		}

		// Validate dir_patterns
		for i, p := range pr.DirPatterns {
			if !validTypes[p.Type] {
				t.Errorf("agent %q: dir_patterns[%d] invalid type %q", agent.Name, i, p.Type)
			}
			if p.Value == "" {
				t.Errorf("agent %q: dir_patterns[%d] empty value", agent.Name, i)
			}
			if p.Type == "regex" {
				if err := validateRegex(p.Value); err != nil {
					t.Errorf("agent %q: dir_patterns[%d] regex %q: %v", agent.Name, i, p.Value, err)
				}
			}
		}

		// version_regex should be valid Go regex if present
		if pr.VersionRegex != "" {
			if err := validateRegex(pr.VersionRegex); err != nil {
				t.Errorf("agent %q: invalid version_regex %q: %v", agent.Name, pr.VersionRegex, err)
			}
		}
	}
}

// TestAgentsYAML_FileRules validates file detection rules consistency.
func TestAgentsYAML_FileRules(t *testing.T) {
	rf := loadAgentsYAML(t)
	validFileTypes := map[string]bool{"file": true, "directory": true}

	for _, agent := range rf.Agents {
		for i, fr := range agent.Files {
			if fr.Path == "" {
				t.Errorf("agent %q: files[%d] empty path", agent.Name, i)
			}

			if !validFileTypes[fr.FileType] {
				t.Errorf("agent %q: files[%d] invalid file_type %q (want file or directory)",
					agent.Name, i, fr.FileType)
			}

			// OS filter must be valid
			if fr.OS != "" && fr.OS != "all" {
				validOS := map[string]bool{"linux": true, "darwin": true, "windows": true}
				if !validOS[fr.OS] {
					t.Errorf("agent %q: files[%d] invalid OS filter %q", agent.Name, i, fr.OS)
				}
			}

			// Directory type with MaxDepth=0 is valid (means "just check existence")
		}
	}
}

// TestAgentsYAML_IDERules validates IDE detection rules consistency.
func TestAgentsYAML_IDERules(t *testing.T) {
	rf := loadAgentsYAML(t)

	for _, agent := range rf.Agents {
		if agent.IDE == nil {
			continue
		}
		ide := agent.IDE

		// Must have scan_paths
		if len(ide.ScanPaths) == 0 {
			t.Errorf("agent %q: IDE rule has no scan_paths", agent.Name)
		}

		// Must have either ext_ids or keywords for matching
		if len(ide.ExtIDs) == 0 && len(ide.Keywords) == 0 {
			t.Errorf("agent %q: IDE rule has neither ext_ids nor keywords", agent.Name)
		}
	}
}

// TestAgentsYAML_SkillRules validates skill discovery rules.
func TestAgentsYAML_SkillRules(t *testing.T) {
	rf := loadAgentsYAML(t)

	for _, agent := range rf.Agents {
		if agent.Skills == nil {
			continue
		}
		sr := agent.Skills

		// scan_paths is optional when auto_discover is enabled (default true);
		// only require it when auto_discover is explicitly disabled.
		adEnabled := sr.AutoDiscover == nil || *sr.AutoDiscover
		if !adEnabled && len(sr.ScanPaths) == 0 {
			t.Errorf("agent %q: skill rule has no scan_paths and auto_discover is disabled", agent.Name)
		}

		// Check defaults applied
		if sr.MaxDepth == 0 {
			t.Errorf("agent %q: skill MaxDepth default not applied (got 0)", agent.Name)
		}
		if sr.MaxSizeKB == 0 {
			t.Errorf("agent %q: skill MaxSizeKB default not applied (got 0)", agent.Name)
		}
	}
}

// TestAgentsYAML_ConfigRules validates config extraction rules.
func TestAgentsYAML_ConfigRules(t *testing.T) {
	rf := loadAgentsYAML(t)
	validFormats := map[string]bool{"json": true, "yaml": true, "yml": true, "env": true, "toml": true}

	for _, agent := range rf.Agents {
		if agent.Config == nil {
			continue
		}
		cfg := agent.Config

		if !validFormats[cfg.Format] {
			t.Errorf("agent %q: invalid config format %q", agent.Name, cfg.Format)
		}

		if len(cfg.Paths) == 0 {
			t.Errorf("agent %q: config rule has no paths", agent.Name)
		}

		if len(cfg.FieldMap) == 0 {
			t.Errorf("agent %q: config rule has no field_map", agent.Name)
		}
	}
}

// TestAgentsYAML_NoDuplicateNames checks for duplicate agent names.
func TestAgentsYAML_NoDuplicateNames(t *testing.T) {
	rf := loadAgentsYAML(t)
	seen := make(map[string]int) // name → first index

	for i, agent := range rf.Agents {
		if firstIdx, exists := seen[agent.Name]; exists {
			t.Errorf("duplicate agent name %q at index %d (first at %d)", agent.Name, i, firstIdx)
		} else {
			seen[agent.Name] = i
		}
	}
}

// TestAgentsYAML_CrossPlatformOSFilters verifies that OS-specific rules are properly tagged.
func TestAgentsYAML_CrossPlatformOSFilters(t *testing.T) {
	rf := loadAgentsYAML(t)

	for _, agent := range rf.Agents {
		for _, fr := range agent.Files {
			if fr.OS != "" && fr.OS != "all" && fr.OS != runtime.GOOS {
				// This rule targets a different OS — on this platform it should be skipped.
				// Verify that there are other detection methods (process/ide) or other
				// files that CAN match on this platform.
				hasOtherFiles := false
				for _, fr2 := range agent.Files {
					if fr2.OS == "" || fr2.OS == "all" || fr2.OS == runtime.GOOS {
						if fr2.Path != fr.Path {
							hasOtherFiles = true
							break
						}
					}
				}
				hasOtherDetection := agent.Process != nil || agent.IDE != nil
				if !hasOtherFiles && !hasOtherDetection {
					t.Logf("agent %q: file rule %q targets %s only, and no other detection methods exist on %s",
						agent.Name, fr.Path, fr.OS, runtime.GOOS)
				}
			}
		}
	}
}

// TestAgentsYAML_RegexPatterns validates all regex patterns compile.
func TestAgentsYAML_RegexPatterns(t *testing.T) {
	rf := loadAgentsYAML(t)

	for _, agent := range rf.Agents {
		if agent.Process != nil {
			pr := agent.Process

			allPatterns := append(pr.NamePatterns, pr.CmdPatterns...)
			allPatterns = append(allPatterns, pr.ExePatterns...)
			allPatterns = append(allPatterns, pr.DirPatterns...)

			for _, p := range allPatterns {
				if p.Type == "regex" && p.Value != "" {
					if err := validateRegex(p.Value); err != nil {
						t.Errorf("agent %q: invalid regex pattern %q: %v", agent.Name, p.Value, err)
					}
				}
			}

			if pr.VersionRegex != "" {
				if err := validateRegex(pr.VersionRegex); err != nil {
					t.Errorf("agent %q: invalid version_regex %q: %v", agent.Name, pr.VersionRegex, err)
				}
			}
		}
	}
}

// TestProcessRules_AndLogicRequiresMultipleFields verifies that "and" logic rules
// have at least two pattern fields (otherwise "and" is equivalent to single-field "or").
func TestProcessRules_AndLogicRequiresMultipleFields(t *testing.T) {
	rf := loadAgentsYAML(t)

	for _, agent := range rf.Agents {
		if agent.Process == nil || agent.Process.MatchLogic != "and" {
			continue
		}
		pr := agent.Process
		fieldCount := 0
		if len(pr.NamePatterns) > 0 {
			fieldCount++
		}
		if len(pr.CmdPatterns) > 0 {
			fieldCount++
		}
		if len(pr.ExePatterns) > 0 {
			fieldCount++
		}
		if len(pr.DirPatterns) > 0 {
			fieldCount++
		}
		if fieldCount < 2 {
			t.Errorf("agent %q: match_logic=and but only %d pattern field(s) defined (need >= 2 for meaningful and)",
				agent.Name, fieldCount)
		}
	}
}

func validateRegex(pattern string) error {
	// Use regexp2 (PCRE-compatible) for validation since the scanner uses it.
	_, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	return nil
}
