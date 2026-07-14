package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/model"
)

// BinaryScanner detects agents via CLI binaries present in PATH.
type BinaryScanner struct{}

// NewBinaryScanner creates a new BinaryScanner.
func NewBinaryScanner() *BinaryScanner {
	return &BinaryScanner{}
}

// Scan runs binary-in-PATH detection for all rules with BinaryRule configured.
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
	cmd := exec.Command(binaryPath, flag)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	output := strings.TrimSpace(string(out))
	re, err := regexp2.Compile(versionRegex, regexp2.None)
	if err != nil {
		return ""
	}

	if m, err := re.FindStringMatch(output); err == nil && m != nil && len(m.Groups()) >= 2 {
		return m.Groups()[1].String()
	}
	return ""
}

func (bs *BinaryScanner) findInPath(pp model.PatternRule) []string {
	re, err := regexp2.Compile(pp.Value, regexp2.None)
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
			matched, err := re.MatchString(base)
			if err != nil || !matched {
				continue
			}
			found = append(found, entry)
		}
	}
	return found
}

func getPathEnv() string {
	path := os.Getenv("PATH")
	if path == "" {
		return "/usr/local/bin:/usr/bin:/bin"
	}
	return path
}
