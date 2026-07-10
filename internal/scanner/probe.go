package scanner

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/model"
)

// ProbeScanner detects agents by executing probe commands and
// extracting type confirmation + version from command output.
type ProbeScanner struct{}

// NewProbeScanner creates a new ProbeScanner.
func NewProbeScanner() *ProbeScanner {
	return &ProbeScanner{}
}

// Scan runs command-based probing for all rules that have ProbeRule configured.
func (ps *ProbeScanner) Scan(rules []model.AgentRule) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent

	for _, rule := range rules {
		if rule.Probe == nil {
			continue
		}
		if agent := ps.probeRule(rule); agent != nil {
			results = append(results, *agent)
		}
	}
	return results
}

func (ps *ProbeScanner) probeRule(rule model.AgentRule) *model.DiscoveredAgent {
	pr := rule.Probe

	// Check if command is available
	cmdPath, err := exec.LookPath(pr.Command)
	if err != nil {
		return nil
	}

	// Build command
	cmd := exec.Command(cmdPath, pr.Args...)
	out, err := cmd.Output()
	output := strings.TrimSpace(string(out))

	// If ExpectedOutput is set, it must appear in output
	if pr.ExpectedOutput != "" {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(pr.ExpectedOutput)) {
			return nil
		}
	}

	// If command failed and ExpectedOutput is empty, we still check
	// if the output has any content (some tools print to stderr)
	if err != nil && pr.ExpectedOutput == "" && output == "" {
		return nil
	}

	agent := model.DiscoveredAgent{
		Name:        rule.Name,
		DisplayName: rule.DisplayName,
		Confidence:  model.Confidence(rule.MinConfidence),
		AssetType:   model.AssetTypeProbe,
		Probe: &model.ProbeInfo{
			Command: fmt.Sprintf("%s %s", pr.Command, strings.Join(pr.Args, " ")),
			Output:  truncateOutput(output, 500),
			Matched: true,
		},
	}

	// Extract version if regex configured
	if pr.VersionRegex != "" {
		if ver := extractProbeVersion(output, pr.VersionRegex); ver != "" {
			agent.Version = ver
		}
	}

	return &agent
}

func extractProbeVersion(output, regex string) string {
	re, err := regexp2.Compile(regex, regexp2.None)
	if err != nil {
		return ""
	}
	if m, err := re.FindStringMatch(output); err == nil && m != nil && len(m.Groups()) >= 2 {
		return m.Groups()[1].String()
	}
	return ""
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
