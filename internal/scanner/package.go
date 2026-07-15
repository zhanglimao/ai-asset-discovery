package scanner

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"

	"github.com/ai-asset-discovery/internal/model"
)

// PackageScanner detects agents via installed packages (npm, pip, apt, etc.).
type PackageScanner struct {
	cache map[string][]pkgInfo // per-manager package list cache
}

// NewPackageScanner creates a new PackageScanner.
func NewPackageScanner() *PackageScanner {
	return &PackageScanner{
		cache: make(map[string][]pkgInfo),
	}
}

// PackageManager defines how to query a specific package manager.
type PackageManager struct {
	Name    string   // npm, pip, apt, brew, cargo, gem, etc.
	Command string   // the executable to run
	ListCmd []string // args to list installed packages, output "name version" per line
}

// Known package managers with their list commands.
// Output convention: one package per line, "name=version" or "name version".
var knownManagers = map[string]PackageManager{
	"npm":   {Name: "npm", Command: "npm", ListCmd: []string{"list", "-g", "--depth=0", "--json"}},
	"pip":   {Name: "pip", Command: "pip", ListCmd: []string{"list", "--format=json"}},
	"pip3":  {Name: "pip3", Command: "pip3", ListCmd: []string{"list", "--format=json"}},
	"apt":   {Name: "apt", Command: "apt", ListCmd: []string{"list", "--installed"}},
	"brew":  {Name: "brew", Command: "brew", ListCmd: []string{"list", "--versions"}},
	"cargo": {Name: "cargo", Command: "cargo", ListCmd: []string{"install", "--list"}},
	"gem":   {Name: "gem", Command: "gem", ListCmd: []string{"list", "--local"}},
}

// Scan runs package detection for all rules that have PackageRule configured.
func (ps *PackageScanner) Scan(rules []model.AgentRule) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent

	for _, rule := range rules {
		if rule.Package == nil {
			continue
		}
		matched := ps.scanPackageRule(rule)
		results = append(results, matched...)
	}
	return results
}

func (ps *PackageScanner) scanPackageRule(rule model.AgentRule) []model.DiscoveredAgent {
	pr := rule.Package
	var results []model.DiscoveredAgent

	for _, mgrName := range pr.Managers {
		mgr, ok := knownManagers[strings.ToLower(mgrName)]
		if !ok {
			continue
		}

		// Check if the package manager itself is available
		if _, err := exec.LookPath(mgr.Command); err != nil {
			continue
		}

		// Query packages
		pkgs := ps.listPackages(mgr)
		for _, pkg := range pkgs {
			if !ps.matchPackage(pkg.Name, pr.Packages) {
				continue
			}

			agent := model.DiscoveredAgent{
				Name:        rule.Name,
				DisplayName: rule.DisplayName,
				Version:     pkg.Version,
				Confidence:  model.Confidence(rule.MinConfidence),
				AssetType:   model.AssetTypePackage,
				Package: &model.PackageInfo{
					Name:    pkg.Name,
					Version: pkg.Version,
					Manager: mgrName,
					Scope:   "global",
				},
			}

			// Try version extraction via regex if configured
			if pr.VersionRegex != "" && agent.Version == "" {
				if ver := ps.extractVersion(pkg.Version, pr.VersionRegex); ver != "" {
					agent.Version = ver
				}
			}

			results = append(results, agent)
		}
	}
	return results
}

type pkgInfo struct {
	Name    string
	Version string
}

func (ps *PackageScanner) listPackages(mgr PackageManager) []pkgInfo {
	if pkgs, ok := ps.cache[mgr.Name]; ok {
		return pkgs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, mgr.Command, mgr.ListCmd...)
	out, err := cmd.Output()
	if err != nil {
		ps.cache[mgr.Name] = nil
		return nil
	}

	var pkgs []pkgInfo

	output := string(out)

	switch mgr.Name {
	case "npm":
		pkgs = ps.parseNPMList(output)
	case "pip", "pip3":
		pkgs = ps.parsePipList(output)
	case "apt":
		pkgs = ps.parseAptList(output)
	case "brew":
		pkgs = ps.parseBrewList(output)
	case "cargo":
		pkgs = ps.parseCargoList(output)
	case "gem":
		pkgs = ps.parseGemList(output)
	default:
		pkgs = ps.parseGenericList(output)
	}

	ps.cache[mgr.Name] = pkgs
	return pkgs
}

func (ps *PackageScanner) parseNPMList(output string) []pkgInfo {
	// npm list -g --depth=0 --json produces JSON output.
	// Try JSON first, then fall back to tree-view format.

	// ── JSON format ──
	if pkgs := parseNPMJSON(output); len(pkgs) > 0 {
		return pkgs
	}

	// ── Tree-view format ──
	// Lines look like:
	//   ├── @scope/package@1.2.3
	//   └── package@1.2.3
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Strip Unicode tree-drawing prefix (├──  or └── ).
		// Use TrimLeft with rune set because the prefix contains multi-byte characters.
		const treeChars = "├─└─│ "
		// Check first rune for tree-drawing chars (not first byte — these are multi-byte UTF-8).
		first, _ := utf8.DecodeRuneInString(line)
		if first != '├' && first != '└' {
			continue
		}
		rest := strings.TrimLeft(line, treeChars)
		// Find the last @ — version never contains @.
		lastAt := strings.LastIndex(rest, "@")
		if lastAt >= 0 && lastAt < len(rest)-1 {
			pkgs = append(pkgs, pkgInfo{
				Name:    rest[:lastAt],
				Version: rest[lastAt+1:],
			})
		}
	}
	return pkgs
}

// parseNPMJSON parses `npm list --json` output.
// The JSON shape is { "dependencies": { "pkg": { "version": "..." }, ... } }.
func parseNPMJSON(output string) []pkgInfo {
	// Quick heuristic: JSON starts with { or [.
	trimmed := strings.TrimSpace(output)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil
	}

	var root struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(trimmed), &root); err != nil {
		return nil
	}

	pkgs := make([]pkgInfo, 0, len(root.Dependencies))
	for name, dep := range root.Dependencies {
		if name != "" {
			pkgs = append(pkgs, pkgInfo{Name: name, Version: dep.Version})
		}
	}
	return pkgs
}

func (ps *PackageScanner) parsePipList(output string) []pkgInfo {
	var raw []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil
	}
	pkgs := make([]pkgInfo, 0, len(raw))
	for _, r := range raw {
		if r.Name != "" {
			pkgs = append(pkgs, pkgInfo{Name: r.Name, Version: r.Version})
		}
	}
	return pkgs
}

func (ps *PackageScanner) parseAptList(output string) []pkgInfo {
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// apt list format: "package/stable,now 1.2.3 amd64"
		if !strings.Contains(line, "/") {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		name := parts[0]
		rest := parts[1]
		fields := strings.Fields(rest)
		version := ""
		if len(fields) >= 1 {
			version = fields[0]
		}
		if name != "" {
			pkgs = append(pkgs, pkgInfo{Name: name, Version: version})
		}
	}
	return pkgs
}

func (ps *PackageScanner) parseBrewList(output string) []pkgInfo {
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// brew list --versions: "package 1.2.3 1.1.0"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkgs = append(pkgs, pkgInfo{Name: parts[0], Version: parts[1]})
		}
	}
	return pkgs
}

func (ps *PackageScanner) parseCargoList(output string) []pkgInfo {
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// cargo install --list: "package v1.2.3:"
		line = strings.TrimSpace(line)
		if strings.Contains(line, " v") && strings.HasSuffix(line, ":") {
			line = strings.TrimSuffix(line, ":")
			nameVer := strings.SplitN(line, " v", 2)
			if len(nameVer) == 2 {
				pkgs = append(pkgs, pkgInfo{Name: nameVer[0], Version: nameVer[1]})
			}
		}
	}
	return pkgs
}

func (ps *PackageScanner) parseGemList(output string) []pkgInfo {
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// gem list --local: "package (1.2.3, 1.1.0)"
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, " ("); idx > 0 {
			name := line[:idx]
			rest := line[idx+2:]
			if endIdx := strings.Index(rest, ")"); endIdx > 0 {
				versions := rest[:endIdx]
				firstVersion := strings.Split(versions, ",")[0]
				pkgs = append(pkgs, pkgInfo{Name: name, Version: strings.TrimSpace(firstVersion)})
			}
		}
	}
	return pkgs
}

func (ps *PackageScanner) parseGenericList(output string) []pkgInfo {
	var pkgs []pkgInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkgs = append(pkgs, pkgInfo{Name: parts[0], Version: parts[1]})
		}
	}
	return pkgs
}

func (ps *PackageScanner) matchPackage(name string, patterns []model.PackagePattern) bool {
	for _, p := range patterns {
		switch p.Type {
		case "exact":
			if name == p.Name {
				return true
			}
		case "regex":
			re, err := regexp2.Compile(p.Name, regexp2.None)
			if err != nil {
				continue
			}
			matched, _ := re.MatchString(name)
			if matched {
				return true
			}
		default:
			if name == p.Name {
				return true
			}
		}
	}
	return false
}

func (ps *PackageScanner) extractVersion(text, versionRegex string) string {
	re, err := regexp2.Compile(versionRegex, regexp2.None)
	if err != nil {
		return ""
	}
	if m, err := re.FindStringMatch(text); err == nil && m != nil && len(m.Groups()) >= 2 {
		return m.Groups()[1].String()
	}
	return ""
}
