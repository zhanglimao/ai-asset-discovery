package scanner

import (
	"fmt"
	"os"
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
	if p.Name == "bash" || p.Name == "sh" || p.Name == "dash" {
		if strings.Contains(p.CmdLine, "/discovery") || strings.Contains(p.CmdLine, "discovery ") {
			return true
		}
	}
	return false
}

func (ps *ProcessScanner) matchProcesses(rule model.AgentRule, procs []model.ProcessInfo) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent
	pr := rule.Process

	for _, proc := range procs {
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

	nameHit := len(pr.NamePatterns) == 0
	cmdHit := len(pr.CmdPatterns) == 0
	exeHit := len(pr.ExePatterns) == 0
	dirHit := len(pr.DirPatterns) == 0

	if len(pr.NamePatterns) > 0 {
		nameHit = checkPatterns(pr.NamePatterns, proc.Name, "name")
	}
	if len(pr.CmdPatterns) > 0 {
		cmdHit = checkPatterns(pr.CmdPatterns, proc.CmdLine, "cmd")
	}
	if len(pr.ExePatterns) > 0 {
		exeHit = checkPatterns(pr.ExePatterns, proc.Executable, "exe")
	}
	if len(pr.DirPatterns) > 0 {
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
		return value == p.Value
	case "contains":
		return strings.Contains(value, p.Value)
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
		return strings.Contains(value, p.Value)
	}
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
