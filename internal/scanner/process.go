package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/model"
)

// ProcessScanner scans running processes and matches against rules.
type ProcessScanner struct{}

// NewProcessScanner creates a new ProcessScanner.
func NewProcessScanner() *ProcessScanner {
	return &ProcessScanner{}
}

// Scan scans all processes and returns matched agents.
func (ps *ProcessScanner) Scan(rules []model.AgentRule) ([]model.DiscoveredAgent, error) {
	procs, err := ps.listProcesses()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	// Filter out our own process and its parent shell to avoid
	// self-detection (e.g. bash wrapping discovery with agent cmdline args).
	selfPID := os.Getpid()
	filtered := make([]model.ProcessInfo, 0, len(procs))
	for _, p := range procs {
		// Skip ourselves
		if p.PID == selfPID {
			continue
		}
		// Skip bash/shell processes whose cmdline contains "/discovery"
		// (the parent wrapper that launched discovery)
		if isShellWrapper(p) {
			continue
		}
		filtered = append(filtered, p)
	}

	var results []model.DiscoveredAgent
	for _, rule := range rules {
		if rule.Process == nil {
			continue
		}
		matched := ps.matchProcesses(rule, filtered)
		results = append(results, matched...)
	}
	return results, nil
}

// isShellWrapper returns true if the process is a shell that wraps discovery.
func isShellWrapper(p model.ProcessInfo) bool {
	// Unix shells
	if p.Name == "bash" || p.Name == "sh" || p.Name == "dash" || p.Name == "zsh" {
		if strings.Contains(p.CmdLine, "/discovery") || strings.Contains(p.CmdLine, "discovery ") {
			return true
		}
	}
	// Windows shells
	if p.Name == "powershell" || p.Name == "pwsh" || p.Name == "cmd" {
		if strings.Contains(p.CmdLine, "discovery") || strings.Contains(p.CmdLine, "go run") {
			return true
		}
	}
	return false
}

// isIDEExtensionProcess returns true if the process executable or cmdline
// indicates it's a subprocess of an IDE extension (VS Code, Cursor, Windsurf,
// etc.). This prevents false positives where non-IDE rules match extension
// helper processes (e.g., chatgpt rule matching OpenAI extension's codex.exe).
// Detection is platform-agnostic: looks for "/extensions/" in the path,
// which works on Windows (backslash normalized to forward slash) and Unix.
func isIDEExtensionProcess(proc model.ProcessInfo) bool {
	// Check executable path for IDE extension directories
	pathsToCheck := []string{proc.Executable, proc.CmdLine, proc.CWD}
	for _, p := range pathsToCheck {
		if p == "" {
			continue
		}
		// Normalize separators for cross-platform matching
		normalized := filepath.ToSlash(p)
		lower := strings.ToLower(normalized)
		for _, ideMarker := range ideExtensionMarkers {
			if strings.Contains(lower, ideMarker) {
				return true
			}
		}
	}
	return false
}

// ideExtensionMarkers are path fragments that indicate an IDE extension
// subprocess.  Using lowercase path fragments that work across platforms.
var ideExtensionMarkers = []string{
	"/extensions/",         // VS Code / Cursor / Windsurf / Trae / etc.
	"/.vscode/",            // VS Code internal
	"/.cursor/",            // Cursor internal
	"/.windsurf/",          // Windsurf internal
	"/.trae/",              // Trae internal
	"code-oss/extensions/", // VS Code OSS on Linux
}

func (ps *ProcessScanner) matchProcesses(rule model.AgentRule, procs []model.ProcessInfo) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent
	pr := rule.Process

	for _, proc := range procs {
		// Cross-platform guard: processes spawned from IDE extension directories
		// should only be matched against ide_extension rules. Without this,
		// a CLI agent rule whose cmdline/name contains "chatgpt" would falsely
		// match the ChatGPT VS Code extension's helper process (codex.exe or
		// equivalent on macOS/Linux).
		if rule.Category != "ide_extension" && isIDEExtensionProcess(proc) {
			continue
		}

		matches := ps.evaluateProcess(pr, proc)
		if len(matches) == 0 {
			continue
		}

		agent := model.DiscoveredAgent{
			Name:        rule.Name,
			DisplayName: rule.DisplayName,
			Confidence:  model.Confidence(rule.MinConfidence),
			AssetType:   model.AssetTypeProcess,
			Process:     &proc,
		}

		// Bump confidence if multiple matches (but not past "possible" from "ghost")
		if len(matches) >= 2 && agent.Confidence == model.ConfidenceGhost {
			agent.Confidence = model.ConfidencePossible
		} else if len(matches) >= 2 {
			agent.Confidence = model.ConfidenceConfirmed
		}

		// Extract version if configured
		if pr.VersionRegex != "" {
			if ver := ps.extractVersion(proc, pr.VersionRegex); ver != "" {
				agent.Version = ver
			}
		}

		results = append(results, agent)
	}
	return results
}

func (ps *ProcessScanner) evaluateProcess(pr *model.ProcessRule, proc model.ProcessInfo) []string {
	// If all pattern groups are empty, skip — nothing to match against.
	if len(pr.NamePatterns) == 0 && len(pr.CmdPatterns) == 0 &&
		len(pr.ExePatterns) == 0 && len(pr.DirPatterns) == 0 {
		return nil
	}

	var matches []string

	// Helper to check patterns
	checkPatterns := func(patterns []model.PatternRule, fieldValue, fieldName string) bool {
		for _, p := range patterns {
			if ps.matchPattern(p, fieldValue) {
				matches = append(matches, fmt.Sprintf("%s:%s=%s", fieldName, p.Type, p.Value))
				return true
			}
		}
		return false
	}

	// Track whether each field was actively checked (had patterns)
	hasName := len(pr.NamePatterns) > 0
	hasCmd := len(pr.CmdPatterns) > 0
	hasExe := len(pr.ExePatterns) > 0
	hasDir := len(pr.DirPatterns) > 0

	nameHit := !hasName
	cmdHit := !hasCmd
	exeHit := !hasExe
	dirHit := !hasDir

	if hasName {
		nameHit = checkPatterns(pr.NamePatterns, proc.Name, "name")
	}
	if hasCmd {
		cmdHit = checkPatterns(pr.CmdPatterns, proc.CmdLine, "cmd")
	}
	if hasExe {
		exeHit = checkPatterns(pr.ExePatterns, proc.Executable, "exe")
	}
	if hasDir {
		dirHit = checkPatterns(pr.DirPatterns, proc.CWD, "cwd")
	}

	if pr.MatchLogic == "and" {
		if nameHit && cmdHit && exeHit && dirHit {
			return matches
		}
		return nil
	}

	// MatchLogic "or" (default)
	if nameHit || cmdHit || exeHit || dirHit {
		return matches
	}
	return nil
}

func (ps *ProcessScanner) matchPattern(p model.PatternRule, value string) bool {
	switch p.Type {
	case "exact":
		return strings.EqualFold(value, p.Value)
	case "word":
		return matchWord(value, p.Value)
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(p.Value))
	case "regex":
		re, err := regexp2.Compile(p.Value, regexp2.None)
		if err != nil {
			return false
		}
		matched, err := re.MatchString(value)
		if err != nil {
			return false
		}
		return matched
	default:
		return strings.Contains(strings.ToLower(value), strings.ToLower(p.Value))
	}
}

// matchWord checks if s contains word as a whole word (delimited by
// non-alphanumeric characters, string boundaries, or case transitions).
func matchWord(s, word string) bool {
	sLower := strings.ToLower(s)
	wLower := strings.ToLower(word)
	if sLower == wLower {
		return true
	}
	idx := 0
	for {
		i := strings.Index(sLower[idx:], wLower)
		if i < 0 {
			return false
		}
		pos := idx + i
		// Check left boundary
		if pos > 0 {
			prev := sLower[pos-1]
			if isWordChar(prev) {
				// Check for CamelCase transition: prev is lowercase,
				// word starts with uppercase (e.g. "openClaw" matches "Claw")
				if !(isLower(prev) && isUpper(s[pos])) {
					idx = pos + 1
					continue
				}
			}
		}
		// Check right boundary
		end := pos + len(wLower)
		if end < len(sLower) {
			next := sLower[end]
			if isWordChar(next) {
				idx = pos + 1
				continue
			}
		}
		return true
	}
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isLower(c byte) bool {
	return c >= 'a' && c <= 'z'
}

func isUpper(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

func (ps *ProcessScanner) extractVersion(proc model.ProcessInfo, versionRegex string) string {
	re, err := regexp2.Compile(versionRegex, regexp2.None)
	if err != nil {
		return ""
	}
	// Search in command line first
	if m, err := re.FindStringMatch(proc.CmdLine); err == nil && m != nil && len(m.Groups()) >= 2 {
		return m.Groups()[1].String()
	}
	if m, err := re.FindStringMatch(proc.Executable); err == nil && m != nil && len(m.Groups()) >= 2 {
		return m.Groups()[1].String()
	}
	return ""
}
