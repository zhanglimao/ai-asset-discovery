package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ai-asset-discovery/internal/model"
	"github.com/ai-asset-discovery/internal/rule"
)

// loadRealRules parses the actual agents.yaml from the project root.
func loadRealRules(t *testing.T) *model.RulesFile {
	t.Helper()
	projectRoot := filepath.Join("..", "..")
	yamlPath := filepath.Join(projectRoot, "rules", "agents.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read agents.yaml: %v", err)
	}
	loader := rule.NewLoader()
	rf, err := loader.Parse(data)
	if err != nil {
		t.Fatalf("parse agents.yaml: %v", err)
	}
	return rf
}

// TestRealRules_ParseAndValidate loads the actual agents.yaml and verifies
// every agent rule can be parsed and has valid structure.
func TestRealRules_ParseAndValidate(t *testing.T) {
	rf := loadRealRules(t)
	t.Logf("Loaded %d agent rules from agents.yaml", len(rf.Agents))

	if rf.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", rf.Version)
	}

	for _, agent := range rf.Agents {
		t.Run(agent.Name, func(t *testing.T) {
			if agent.Name == "" {
				t.Error("empty agent name")
			}
			if agent.DisplayName == "" {
				t.Error("empty display_name")
			}
			switch agent.MinConfidence {
			case "confirmed", "possible", "ghost":
			default:
				t.Errorf("invalid min_confidence: %q", agent.MinConfidence)
			}
			hasProcess := agent.Process != nil
			hasFiles := len(agent.Files) > 0
			hasIDE := agent.IDE != nil
			// Check if agent has any detection method (simplified or legacy)
			hasDetection := hasProcess || hasFiles || hasIDE ||
				agent.Features != nil || len(agent.Paths) > 0 ||
				agent.Probe != nil || agent.Package != nil ||
				agent.Binary != nil
			if !hasDetection {
				t.Error("no detection method (process, files, or IDE)")
			}
			if agent.Skills != nil {
				// Skill rule is valid; no keywords required (SKILL.md-only matching)
			}
		})
	}
}

// TestRealRules_ProcessRuleStructure validates process rule syntax.
func TestRealRules_ProcessRuleStructure(t *testing.T) {
	rf := loadRealRules(t)
	validTypes := map[string]bool{"exact": true, "contains": true, "regex": true, "word": true}

	for _, agent := range rf.Agents {
		if agent.Process == nil {
			continue
		}
		pr := agent.Process
		t.Run(agent.Name, func(t *testing.T) {
			if pr.MatchLogic != "" && pr.MatchLogic != "and" && pr.MatchLogic != "or" {
				t.Errorf("invalid match_logic: %q", pr.MatchLogic)
			}
			hasPatterns := len(pr.NamePatterns) > 0 || len(pr.CmdPatterns) > 0 ||
				len(pr.ExePatterns) > 0 || len(pr.DirPatterns) > 0
			if !hasPatterns {
				t.Error("process rule has no patterns")
			}
			checkPatterns := func(patterns []model.PatternRule, fieldName string) {
				for i, p := range patterns {
					if !validTypes[p.Type] {
						t.Errorf("%s[%d]: invalid type %q", fieldName, i, p.Type)
					}
					if p.Value == "" {
						t.Errorf("%s[%d]: empty value", fieldName, i)
					}
				}
			}
			checkPatterns(pr.NamePatterns, "name_patterns")
			checkPatterns(pr.CmdPatterns, "cmd_patterns")
			checkPatterns(pr.ExePatterns, "exe_patterns")
			checkPatterns(pr.DirPatterns, "dir_patterns")
			if pr.MatchLogic == "and" {
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
					t.Errorf("match_logic=and but only %d pattern field(s); need >=2", fieldCount)
				}
			}
		})
	}
}

// TestRealRules_FileRuleStructure validates file rule syntax.
func TestRealRules_FileRuleStructure(t *testing.T) {
	rf := loadRealRules(t)
	for _, agent := range rf.Agents {
		for i, fr := range agent.Files {
			label := fmt.Sprintf("%s/files[%d]", agent.Name, i)
			t.Run(label, func(t *testing.T) {
				if fr.Path == "" {
					t.Error("empty path")
				}
				if fr.FileType != "file" && fr.FileType != "directory" {
					t.Errorf("invalid file_type: %q", fr.FileType)
				}
				if fr.MaxDepth > 0 && fr.FileType == "file" {
					t.Error("max_depth set on file type (only meaningful for directories)")
				}
				if fr.OS != "" && fr.OS != "all" && fr.OS != "linux" && fr.OS != "darwin" && fr.OS != "windows" {
					t.Errorf("invalid OS filter: %q", fr.OS)
				}
			})
		}
	}
}

// TestRealRules_IDERuleStructure validates IDE rule syntax.
func TestRealRules_IDERuleStructure(t *testing.T) {
	rf := loadRealRules(t)
	for _, agent := range rf.Agents {
		if agent.IDE == nil {
			continue
		}
		t.Run(agent.Name, func(t *testing.T) {
			ide := agent.IDE
			// Must have scan_paths
			if len(ide.ScanPaths) == 0 {
				t.Error("IDE rule has no scan_paths")
			}
			if len(ide.ExtIDs) == 0 && len(ide.Keywords) == 0 {
				t.Error("IDE rule has neither ext_ids nor keywords")
			}
		})
	}
}

// TestRealRules_ConfigRuleStructure validates config extraction rules.
func TestRealRules_ConfigRuleStructure(t *testing.T) {
	rf := loadRealRules(t)
	validFormats := map[string]bool{"json": true, "yaml": true, "yml": true, "env": true, "toml": true}
	for _, agent := range rf.Agents {
		if agent.Config == nil {
			continue
		}
		t.Run(agent.Name, func(t *testing.T) {
			if !validFormats[agent.Config.Format] {
				t.Errorf("invalid config format: %q", agent.Config.Format)
			}
			if len(agent.Config.Paths) == 0 {
				t.Error("config rule has no paths")
			}
		})
	}
}

// TestRealRules_SkillRuleStructure validates skill discovery rule defaults.
func TestRealRules_SkillRuleStructure(t *testing.T) {
	rf := loadRealRules(t)
	for _, agent := range rf.Agents {
		if agent.Skills == nil {
			continue
		}
		t.Run(agent.Name, func(t *testing.T) {
			sr := agent.Skills
			if sr.MaxDepth == 0 {
				t.Error("MaxDepth default not applied")
			}
			if sr.MaxSizeKB == 0 {
				t.Error("MaxSizeKB default not applied")
			}
			// Extensions removed — only SKILL.md is matched now
			// scan_paths is optional when auto_discover is enabled (default true);
			// only require it when auto_discover is explicitly disabled.
			adEnabled := sr.AutoDiscover == nil || *sr.AutoDiscover
			if !adEnabled && len(sr.ScanPaths) == 0 {
				t.Error("no scan_paths and auto_discover is disabled")
			}
		})
	}
}

// TestRealRules_NoDuplicateNames ensures no two agents share the same name.
func TestRealRules_NoDuplicateNames(t *testing.T) {
	rf := loadRealRules(t)
	seen := make(map[string]int)
	for i, agent := range rf.Agents {
		if first, ok := seen[agent.Name]; ok {
			t.Errorf("duplicate agent name %q at index %d (first at %d)", agent.Name, i, first)
		}
		seen[agent.Name] = i
	}
}

// TestRealRules_EveryAgentHasDetection checks cross-platform completeness.
func TestRealRules_EveryAgentHasDetection(t *testing.T) {
	rf := loadRealRules(t)
	for _, agent := range rf.Agents {
		hasProcess := agent.Process != nil
		hasFiles := len(agent.Files) > 0
		hasIDE := agent.IDE != nil
		// Check if agent has any detection method (simplified or legacy)
		hasDetection := hasProcess || hasFiles || hasIDE ||
			agent.Features != nil || len(agent.Paths) > 0 ||
			agent.Probe != nil || agent.Package != nil ||
			agent.Binary != nil
		if !hasDetection {
			t.Errorf("agent %q has no detection method", agent.Name)
		}
		if agent.Skills != nil && !hasDetection {
			t.Errorf("agent %q has skills but no detection method", agent.Name)
		}
	}
}

// TestRealRules_VersionRegexHasCaptureGroup tests version_regex has capture groups.
func TestRealRules_VersionRegexHasCaptureGroup(t *testing.T) {
	rf := loadRealRules(t)
	for _, agent := range rf.Agents {
		if agent.Process == nil || agent.Process.VersionRegex == "" {
			continue
		}
		vre := agent.Process.VersionRegex
		if !containsStr(vre, "(") {
			t.Errorf("agent %q: version_regex %q has no capture group", agent.Name, vre)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
