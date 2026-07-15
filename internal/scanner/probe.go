package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

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
// Multiple probes execute concurrently using a bounded goroutine pool.
func (ps *ProbeScanner) Scan(rules []model.AgentRule) []model.DiscoveredAgent {
	// Collect rules with probes
	type probeItem struct {
		rule  model.AgentRule
		index int
	}
	var items []probeItem
	for i := range rules {
		if rules[i].Probe != nil {
			items = append(items, probeItem{rule: rules[i], index: i})
		}
	}
	if len(items) == 0 {
		return nil
	}

	concurrency := max(1, runtime.NumCPU())
	sem := make(chan struct{}, concurrency)
	results := make([]*model.DiscoveredAgent, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, rule model.AgentRule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = ps.probeRule(rule)
		}(i, item.rule)
	}
	wg.Wait()

	var out []model.DiscoveredAgent
	for _, r := range results {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out
}

func (ps *ProbeScanner) probeRule(rule model.AgentRule) *model.DiscoveredAgent {
	pr := rule.Probe

	// Check if command is available
	cmdPath, err := exec.LookPath(pr.Command)
	if err != nil {
		return nil
	}

	// Build command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, pr.Args...)
	// Capture both stdout and stderr — many CLI tools (Python packages,
	// Rust CLIs, npm packages) write version info to stderr.
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	// Combine stdout + stderr so version extraction works regardless of
	// which stream the tool writes to.
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		// Fall back to stderr if stdout is empty.
		output = strings.TrimSpace(stderr.String())
	} else if strings.TrimSpace(stderr.String()) != "" {
		// Append stderr content for more complete output capture.
		output = output + "\n" + strings.TrimSpace(stderr.String())
	}

	// If ExpectedOutput is set, it must appear in combined output
	if pr.ExpectedOutput != "" {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(pr.ExpectedOutput)) {
			return nil
		}
	}

	// If command failed and ExpectedOutput is empty, we still check
	// if the output has any content (some tools print to stderr)
	if runErr != nil && pr.ExpectedOutput == "" && output == "" {
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
	re, err := regexp.Compile(regex)
	if err != nil {
		return ""
	}
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
