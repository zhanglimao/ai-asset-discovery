package scanner

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"regexp"

	"github.com/ai-asset-discovery/internal/model"
)

// PackageScanner detects agents via installed packages (npm, pip, apt, etc.).
type PackageScanner struct {
	cache    map[string][]pkgInfo // per-manager package list cache
	cacheMu  sync.Mutex
	once     sync.Once                 // warmCache runs exactly once
	warmed   bool                      // set to true after warmCache completes
	managers map[string]PackageManager // manager definitions (built-in + rule-defined)
}

// NewPackageScanner creates a new PackageScanner with built-in defaults.
func NewPackageScanner() *PackageScanner {
	return &PackageScanner{
		cache:    make(map[string][]pkgInfo),
		managers: defaultPackageManagers(),
	}
}

// PackageManager defines how to query a specific package manager.
type PackageManager struct {
	Name         string   // npm, pip, apt, brew, cargo, gem, etc.
	Command      string   // the executable to run
	ListCmd      []string // args to list installed packages
	OutputFormat string   // parsing strategy: json_npm, json_pip, text_apt, etc.
	Timeout      int      // timeout in seconds (default 3)
}

// SetManagers replaces the scanner's manager definitions with the given set.
// Called by the engine when custom package_managers are defined in rules.
func (ps *PackageScanner) SetManagers(mgrs map[string]PackageManager) {
	ps.managers = mgrs
}

// RegisterManager adds or replaces a single package manager definition.
func (ps *PackageScanner) RegisterManager(name string, mgr PackageManager) {
	if ps.managers == nil {
		ps.managers = make(map[string]PackageManager)
	}
	mgr.Name = name
	ps.managers[name] = mgr
}

// defaultPackageManagers returns the built-in package manager definitions.
// These serve as defaults when no custom package_managers are specified in rules.
func defaultPackageManagers() map[string]PackageManager {
	return map[string]PackageManager{
		"npm":   {Name: "npm", Command: "npm", ListCmd: []string{"list", "-g", "--depth=0", "--json"}, OutputFormat: "json_npm", Timeout: 3},
		"pip":   {Name: "pip", Command: "pip", ListCmd: []string{"list", "--format=json"}, OutputFormat: "json_pip", Timeout: 3},
		"pip3":  {Name: "pip3", Command: "pip3", ListCmd: []string{"list", "--format=json"}, OutputFormat: "json_pip", Timeout: 3},
		"apt":   {Name: "apt", Command: "apt", ListCmd: []string{"list", "--installed"}, OutputFormat: "text_apt", Timeout: 3},
		"brew":  {Name: "brew", Command: "brew", ListCmd: []string{"list", "--versions"}, OutputFormat: "text_brew", Timeout: 3},
		"cargo": {Name: "cargo", Command: "cargo", ListCmd: []string{"install", "--list"}, OutputFormat: "text_cargo", Timeout: 3},
		"gem":   {Name: "gem", Command: "gem", ListCmd: []string{"list", "--local"}, OutputFormat: "text_gem", Timeout: 3},
	}
}

// Scan runs package detection for all rules that have PackageRule configured.
func (ps *PackageScanner) Scan(rules []model.AgentRule) []model.DiscoveredAgent {
	// Eagerly warm all package manager caches in parallel before scanning rules.
	ps.warmCache()

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

// warmCache runs all available package manager list commands in parallel
// and populates the cache. Safe to call multiple times — only the first
// call does work.
func (ps *PackageScanner) warmCache() {
	ps.once.Do(func() {
		// Collect available managers (LookPath check is fast, do it sequentially first)
		type mgrEntry struct {
			key string
			mgr PackageManager
		}
		var available []mgrEntry
		for key, mgr := range ps.managers {
			if _, err := exec.LookPath(mgr.Command); err == nil {
				available = append(available, mgrEntry{key: key, mgr: mgr})
			}
		}
		if len(available) == 0 {
			ps.warmed = true
			return
		}

		// Run list commands in parallel
		type listResult struct {
			key  string
			pkgs []pkgInfo
		}
		resultCh := make(chan listResult, len(available))
		var wg sync.WaitGroup

		for _, entry := range available {
			wg.Add(1)
			go func(key string, mgr PackageManager) {
				defer wg.Done()
				pkgs := ps.queryPackageManager(mgr)
				resultCh <- listResult{key: key, pkgs: pkgs}
			}(entry.key, entry.mgr)
		}

		go func() {
			wg.Wait()
			close(resultCh)
		}()

		ps.cacheMu.Lock()
		for r := range resultCh {
			ps.cache[r.key] = r.pkgs
		}
		ps.warmed = true
		ps.cacheMu.Unlock()
	})
}

func (ps *PackageScanner) queryPackageManager(mgr PackageManager) []pkgInfo {
	timeoutSec := mgr.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, mgr.Command, mgr.ListCmd...)
	// Capture both stdout and stderr. Package managers sometimes exit
	// with non-zero status while still producing valid output on stdout
	// (e.g. pip deprecation warnings, npm peer-dependency warnings).
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	output := stdout.String()
	if output == "" {
		// Some tools (rare) may write structured data to stderr.
		output = stderr.String()
	}
	if output == "" {
		return nil
	}

	return parsePackageList(mgr.OutputFormat, output)
}

func (ps *PackageScanner) scanPackageRule(rule model.AgentRule) []model.DiscoveredAgent {
	pr := rule.Package
	var results []model.DiscoveredAgent

	for _, mgrName := range pr.Managers {
		mgr, ok := ps.managers[strings.ToLower(mgrName)]
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
	ps.cacheMu.Lock()
	if pkgs, ok := ps.cache[mgr.Name]; ok || ps.warmed {
		ps.cacheMu.Unlock()
		return pkgs
	}
	ps.cacheMu.Unlock()

	// Fallback: if warmCache wasn't called, query synchronously
	pkgs := ps.queryPackageManager(mgr)
	ps.cacheMu.Lock()
	ps.cache[mgr.Name] = pkgs
	ps.cacheMu.Unlock()
	return pkgs
}

func (ps *PackageScanner) parseNPMList(output string) []pkgInfo {
	return parseNPMList(output)
}

// parseNPMList parses `npm list` output (JSON or tree-view format).
func parseNPMList(output string) []pkgInfo {
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

func parsePipList(output string) []pkgInfo {
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

func parseAptList(output string) []pkgInfo {
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

func parseBrewList(output string) []pkgInfo {
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

func parseCargoList(output string) []pkgInfo {
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

func parseGemList(output string) []pkgInfo {
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

func parseGenericList(output string) []pkgInfo {
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

// parsePackageList dispatches to the appropriate parser based on output_format.
// This is the single entry point for parsing package manager output — no
// manager-specific logic is hardcoded in the scanner, everything is driven
// by the output_format field from the manager definition.
func parsePackageList(format, output string) []pkgInfo {
	switch format {
	case "json_npm":
		return parseNPMList(output)
	case "json_pip":
		return parsePipList(output)
	case "text_apt":
		return parseAptList(output)
	case "text_brew":
		return parseBrewList(output)
	case "text_cargo":
		return parseCargoList(output)
	case "text_gem":
		return parseGemList(output)
	case "text_generic":
		return parseGenericList(output)
	default:
		// Unknown format — try JSON first (most common), then generic text.
		if pkgs := parseNPMList(output); len(pkgs) > 0 {
			return pkgs
		}
		if pkgs := parsePipList(output); len(pkgs) > 0 {
			return pkgs
		}
		return parseGenericList(output)
	}
}

func (ps *PackageScanner) matchPackage(name string, patterns []model.PackagePattern) bool {
	for _, p := range patterns {
		switch p.Type {
		case "exact":
			if name == p.Name {
				return true
			}
		case "regex":
			re, err := regexp.Compile(p.Name)
			if err != nil {
				continue
			}
			if re.MatchString(name) {
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
	re, err := regexp.Compile(versionRegex)
	if err != nil {
		return ""
	}
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
