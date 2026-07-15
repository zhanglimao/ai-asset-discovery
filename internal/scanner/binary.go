package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ai-asset-discovery/internal/model"
)

// BinaryScanner detects agents via CLI binaries present in PATH.
type BinaryScanner struct{}

// NewBinaryScanner creates a new BinaryScanner.
func NewBinaryScanner() *BinaryScanner {
	return &BinaryScanner{}
}

// Scan runs binary-in-PATH detection for all rules with BinaryRule configured.
// Version extraction happens inline in scanBinaryRule/buildAgent (which calls
// getVersion synchronously per binary). This is intentional: LookPath is the
// fast part, and version extraction is I/O-bound on the target binary, so
// parallelizing the discovery phase first would not help — buildAgent already
// has the binary path and extracts the version immediately.
func (bs *BinaryScanner) Scan(rules []model.AgentRule) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent
	for _, rule := range rules {
		if rule.Binary == nil {
			continue
		}
		matched := bs.scanBinaryRule(rule)
		results = append(results, matched...)
	}
	return results
}

func (bs *BinaryScanner) scanBinaryRule(rule model.AgentRule) []model.DiscoveredAgent {
	br := rule.Binary
	var results []model.DiscoveredAgent

	for _, pp := range br.Names {
		candidate := pp.Value

		// For regex patterns, walk PATH directories to find matches.
		// Regex patterns can't be used with exec.LookPath.
		if pp.Type == "regex" {
			found := bs.findInPath(pp)
			for _, fp := range found {
				agent := bs.buildAgent(rule, fp, br)
				results = append(results, agent)
			}
			continue
		}

		// Try LookPath for exact or contains patterns
		binaryPath, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		absPath, _ := filepath.Abs(binaryPath)
		agent := bs.buildAgent(rule, absPath, br)
		results = append(results, agent)
	}

	return results
}

func (bs *BinaryScanner) buildAgent(rule model.AgentRule, binaryPath string, br *model.BinaryRule) model.DiscoveredAgent {
	binName := filepath.Base(binaryPath)

	agent := model.DiscoveredAgent{
		Name:        rule.Name,
		DisplayName: rule.DisplayName,
		Confidence:  model.Confidence(rule.MinConfidence),
		AssetType:   model.AssetTypeBinary,
		Binary: &model.BinaryInfo{
			Name: binName,
			Path: binaryPath,
		},
	}

	// Try to extract version if flag and regex configured
	if br.VersionFlag != "" && br.VersionRegex != "" {
		if ver := bs.getVersion(binaryPath, br.VersionFlag, br.VersionRegex); ver != "" {
			agent.Binary.Version = ver
			agent.Version = ver
		}
	}

	return agent
}

func (bs *BinaryScanner) getVersion(binaryPath, flag, versionRegex string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, flag)
	// Capture both stdout and stderr — many CLI tools write version
	// info to stderr (Python packages, Rust CLIs, etc.).
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	// Try stdout first, fall back to stderr.
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	if output == "" {
		return ""
	}

	re, err := regexp.Compile(versionRegex)
	if err != nil {
		return ""
	}

	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func (bs *BinaryScanner) findInPath(pp model.PatternRule) []string {
	re, err := regexp.Compile(pp.Value)
	if err != nil {
		return nil
	}

	// Walk PATH directories and match against filenames
	pathEnv := getPathEnv()
	var found []string
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := filepath.Glob(filepath.Join(dir, "*"))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			base := filepath.Base(entry)
			if !re.MatchString(base) {
				continue
			}
			found = append(found, entry)
		}
	}
	return found
}

func getPathEnv() string {
	path := os.Getenv("PATH")
	if path != "" {
		return path
	}
	// Platform-appropriate fallback when PATH is unset.
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32;C:\Windows;C:\Windows\System32\Wbem`
	}
	return "/usr/local/bin:/usr/bin:/bin"
}
