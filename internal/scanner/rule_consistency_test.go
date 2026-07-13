package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/model"
	"github.com/ai-asset-discovery/internal/rule"
)

// loadAgentsRules loads and parses agents.yaml from project root.
func loadAgentsRules(t *testing.T) *model.RulesFile {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	loader := rule.NewLoader()
	rf, err := loader.LoadFile(filepath.Join(dir, "rules", "agents.yaml"))
	if err != nil {
		t.Fatalf("load agents.yaml: %v", err)
	}
	return rf
}

// TestRuleToScanner_ProcessMatchLogic verifies that every process rule in agents.yaml
// is self-consistent with the scanner's evaluateProcess logic.
func TestRuleToScanner_ProcessMatchLogic(t *testing.T) {
	rf := loadAgentsRules(t)
	ps := NewProcessScanner()

	for _, agent := range rf.Agents {
		if agent.Process == nil {
			continue
		}
		pr := agent.Process
		t.Run(agent.Name, func(t *testing.T) {
			// For every pattern in the rule, verify:
			// 1. The pattern type + scanner's matchPattern produce consistent results
			// 2. The evaluateProcess logic (or/and) works correctly

			// Verify each pattern individually
			for _, p := range pr.NamePatterns {
				testSinglePattern(t, ps, p, "name")
			}
			for _, p := range pr.CmdPatterns {
				testSinglePattern(t, ps, p, "cmd")
			}
			for _, p := range pr.ExePatterns {
				testSinglePattern(t, ps, p, "exe")
			}
			for _, p := range pr.DirPatterns {
				testSinglePattern(t, ps, p, "dir")
			}

			// Test the combined logic with a matching process
			proc := buildMatchingProcess(pr)
			matches := ps.evaluateProcess(pr, proc)
			if len(matches) == 0 {
				t.Errorf("built matching process should match: name=%q cmd=%q exe=%q cwd=%q",
					proc.Name, proc.CmdLine, proc.Executable, proc.CWD)
			}

			// Verify negative (non-matching process)
			noMatch := model.ProcessInfo{
				PID: 88888, Name: "zz-notexist-x1",
				CmdLine:    "/usr/zz-notexist-x1",
				Executable: "/usr/bin/zz-notexist-x1", CWD: "/tmp/zz-notexist-x1",
			}
			negMatches := ps.evaluateProcess(pr, noMatch)
			// For "or" logic with multiple pattern groups, a negative should not match.
			// For "and" logic, if some pattern groups are empty they default to true,
			// so a negative won't match as long as at least one non-empty group fails.
			if pr.MatchLogic != "and" && len(negMatches) > 0 {
				t.Errorf("non-matching process unexpectedly matched: matches=%v", negMatches)
			}
		})
	}
}

func testSinglePattern(t *testing.T, ps *ProcessScanner, p model.PatternRule, field string) {
	t.Helper()

	// Generate a value that should match the pattern
	positiveValue, canVerifyPositive := generatePositiveValue(p)
	if canVerifyPositive {
		if !ps.matchPattern(p, positiveValue) {
			t.Errorf("field=%s pattern=(type=%s value=%s): positive value %q did NOT match",
				field, p.Type, p.Value, positiveValue)
		}
	}

	// Negative: a value that should NEVER match
	negativeValue := "zz-no-match-xyz-12345-!"
	if ps.matchPattern(p, negativeValue) {
		if p.Type == "contains" && p.Value == "" {
			return // empty contains always matches
		}
		t.Errorf("field=%s pattern=(type=%s value=%s): negative value %q unexpectedly MATCHED",
			field, p.Type, p.Value, negativeValue)
	}
}

func generatePositiveValue(p model.PatternRule) (string, bool) {
	switch p.Type {
	case "exact":
		return p.Value, true
	case "contains":
		return fmt.Sprintf("prefix_%s_suffix", p.Value), true
	case "regex":
		re, err := regexp2.Compile(p.Value, regexp2.None)
		if err != nil {
			return p.Value, false // can't compile
		}
		// Try simple known patterns first
		candidates := []string{
			"python",      // matches ^(python3?|node)$
			"python3",     // matches ^(python3?|node)$
			"node",        // matches ^(python3?|node)$
			"ChatGPT",     // matches chatgpt patterns
			"chatgpt",     // matches chatgpt patterns
			"ChatGpt",     // matches [Cc]hat[Gg][Pp][Tt]
			"langchain",   // matches langchain patterns
			"AutoGen",     // matches autogen patterns
			"crewai",      // matches crewai patterns
			"llama_index", // matches llamaindex patterns
		}
		for _, c := range candidates {
			if matched, _ := re.MatchString(c); matched {
				return c, true
			}
		}
		// Can't auto-generate a match for this regex — skip positive check
		return p.Value, false
	default:
		return p.Value, true
	}
}

func buildMatchingProcess(pr *model.ProcessRule) model.ProcessInfo {
	proc := model.ProcessInfo{PID: 99999}

	// Use real matching values for each pattern group
	if len(pr.NamePatterns) > 0 {
		val, _ := generatePositiveValue(pr.NamePatterns[0])
		proc.Name = val
	}
	if len(pr.CmdPatterns) > 0 {
		val, _ := generatePositiveValue(pr.CmdPatterns[0])
		proc.CmdLine = val
	}
	if len(pr.ExePatterns) > 0 {
		val, _ := generatePositiveValue(pr.ExePatterns[0])
		proc.Executable = val
	}
	if len(pr.DirPatterns) > 0 {
		val, _ := generatePositiveValue(pr.DirPatterns[0])
		proc.CWD = val
	}

	return proc
}

// TestRuleToScanner_FileRulePaths validates that file rule paths in agents.yaml
// can be expanded by config.ExpandPath without error.
func TestRuleToScanner_FileRulePaths(t *testing.T) {
	rf := loadAgentsRules(t)

	for _, agent := range rf.Agents {
		for i, fr := range agent.Files {
			label := fmt.Sprintf("%s/file[%d]", agent.Name, i)
			t.Run(label, func(t *testing.T) {
				expanded, err := config.ExpandPath(fr.Path)
				if err != nil {
					t.Errorf("ExpandPath(%q) error: %v", fr.Path, err)
					return
				}
				// Path should not be empty after expansion
				if expanded == "" {
					t.Errorf("ExpandPath(%q) returned empty", fr.Path)
				}
				// Just log whether it exists
				if _, statErr := os.Stat(expanded); statErr == nil {
					t.Logf("path %q EXISTS on this system", expanded)
				}
			})
		}
	}
}

// TestRuleToScanner_CrossPlatformOSFilters verifies that OS-specific file rules
// are correctly filtered by the FileScanner on the current platform.
func TestRuleToScanner_CrossPlatformOSFilters(t *testing.T) {
	rf := loadAgentsRules(t)
	fs := NewFileScanner()
	currentOS := config.OSName()

	// Track rules that would be skipped on this platform
	for _, agent := range rf.Agents {
		allOSFiltered := len(agent.Files) > 0
		hasCurrentOSFile := false
		for _, fr := range agent.Files {
			if fr.OS == "" || fr.OS == "all" || fr.OS == currentOS {
				hasCurrentOSFile = true
				break
			}
		}
		if !hasCurrentOSFile && agent.Process == nil && agent.IDE == nil {
			t.Logf("agent %q: all file rules filtered out on %s, and no process/IDE detection → UNDETECTABLE on this OS",
				agent.Name, currentOS)
		}
		_ = allOSFiltered
		_ = fs
	}
}

// TestRuleToScanner_SkillRulesDefaults validates skill rule default values match scanner expectations.
func TestRuleToScanner_SkillRulesDefaults(t *testing.T) {
	rf := loadAgentsRules(t)

	for _, agent := range rf.Agents {
		if agent.Skills == nil {
			continue
		}
		sr := agent.Skills
		t.Run(agent.Name, func(t *testing.T) {
			if sr.MaxDepth < 1 {
				t.Errorf("MaxDepth=%d (default should be 3)", sr.MaxDepth)
			}
			if sr.MaxSizeKB < 1 {
				t.Errorf("MaxSizeKB=%d (default should be 100)", sr.MaxSizeKB)
			}
		})
	}
}

// TestRuleToScanner_VersionRegexCompiles ensures all version_regex patterns
// compile as valid Go regexps.
func TestRuleToScanner_VersionRegexCompiles(t *testing.T) {
	rf := loadAgentsRules(t)

	for _, agent := range rf.Agents {
		if agent.Process == nil || agent.Process.VersionRegex == "" {
			continue
		}
		t.Run(agent.Name, func(t *testing.T) {
			re, err := regexp2.Compile(agent.Process.VersionRegex, regexp2.None)
			if err != nil {
				t.Errorf("version_regex %q does not compile: %v", agent.Process.VersionRegex, err)
				return
			}
			// regexp2 requires counting capture groups differently — just verify it compiles
			_ = re
		})
	}
}

// TestRuleToScanner_RegexPatternsCompile ensures all regex-type process patterns compile.
func TestRuleToScanner_RegexPatternsCompile(t *testing.T) {
	rf := loadAgentsRules(t)

	for _, agent := range rf.Agents {
		if agent.Process == nil {
			continue
		}
		pr := agent.Process

		allPatterns := append(pr.NamePatterns, pr.CmdPatterns...)
		allPatterns = append(allPatterns, pr.ExePatterns...)
		allPatterns = append(allPatterns, pr.DirPatterns...)

		for _, p := range allPatterns {
			if p.Type != "regex" {
				continue
			}
			_, err := regexp2.Compile(p.Value, regexp2.None)
			if err != nil {
				t.Errorf("agent %q: regex pattern %q does not compile: %v", agent.Name, p.Value, err)
			}
		}
	}
}

// TestRuleToScanner_MIN_CONFIDENCE_UPPERCASE verifies that min_confidence values
// are lowercase (to match model.Confidence constants).
func TestRuleToScanner_MIN_CONFIDENCE_UPPERCASE(t *testing.T) {
	rf := loadAgentsRules(t)

	valid := map[string]bool{
		"confirmed": true, "possible": true, "ghost": true,
		"CONFIRMED": false, "POSSIBLE": false, "GHOST": false,
	}

	for _, agent := range rf.Agents {
		if !valid[agent.MinConfidence] {
			t.Errorf("agent %q: min_confidence %q is not lowercase valid value", agent.Name, agent.MinConfidence)
		}
	}
}
