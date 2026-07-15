package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/ide"
	"github.com/ai-asset-discovery/internal/model"
	"github.com/ai-asset-discovery/internal/rule"
	"github.com/ai-asset-discovery/internal/scanner"
	"github.com/ai-asset-discovery/internal/skill"
)

// Engine orchestrates AI asset discovery.
type Engine struct {
	processScanner *scanner.ProcessScanner
	fileScanner    *scanner.FileScanner
	ideScanner     *ide.Scanner
	pkgScanner     *scanner.PackageScanner
	binaryScanner  *scanner.BinaryScanner
	probeScanner   *scanner.ProbeScanner
	skillDiscover  *skill.Discoverer
	ruleLoader     *rule.Loader
	rules          *model.RulesFile
}

// Result holds the full discovery output.
type Result struct {
	Version string                  `json:"version"`
	Summary Summary                 `json:"summary"`
	Agents  []model.DiscoveredAgent `json:"agents"`
}

// Summary of the discovery run.
type Summary struct {
	TotalAgents     int            `json:"total_agents"`
	ConfirmedAgents int            `json:"confirmed_agents"`
	PossibleAgents  int            `json:"possible_agents"`
	GhostAgents     int            `json:"ghost_agents"`
	TotalSkills     int            `json:"total_skills"`
	ByType          map[string]int `json:"by_type"`
	Errors          []string       `json:"errors,omitempty"`
}

// NewEngine creates a new discovery engine.
func NewEngine() *Engine {
	return &Engine{
		processScanner: scanner.NewProcessScanner(),
		fileScanner:    scanner.NewFileScanner(),
		ideScanner:     ide.NewScanner(),
		pkgScanner:     scanner.NewPackageScanner(),
		binaryScanner:  scanner.NewBinaryScanner(),
		probeScanner:   scanner.NewProbeScanner(),
		skillDiscover:  skill.NewDiscoverer(),
		ruleLoader:     rule.NewLoader(),
	}
}

// LoadRules loads rules from a file or directory.
func (e *Engine) LoadRules(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("rules path %s: %w", path, err)
	}
	if info.IsDir() {
		e.rules, err = e.ruleLoader.LoadDir(path)
	} else {
		e.rules, err = e.ruleLoader.LoadFile(path)
	}
	return err
}

// LoadRulesFromBytes loads rules directly from YAML bytes.
func (e *Engine) LoadRulesFromBytes(data []byte) error {
	var err error
	e.rules, err = e.ruleLoader.Parse(data)
	return err
}

// Run executes the full discovery process.
func (e *Engine) Run() (*Result, error) {
	if e.rules == nil {
		return nil, fmt.Errorf("no rules loaded")
	}

	result := &Result{
		Version: "1.0",
		Summary: Summary{
			ByType: make(map[string]int),
		},
	}

	var errors []string

	// Phase 1: Process scanning
	processAgents, err := e.processScanner.Scan(e.rules.Agents)
	if err != nil {
		errors = append(errors, fmt.Sprintf("process scan: %v", err))
	} else {
		result.Agents = append(result.Agents, processAgents...)
	}

	// Phase 2: File scanning
	fileAgents, err := e.fileScanner.Scan(e.rules.Agents)
	if err != nil {
		errors = append(errors, fmt.Sprintf("file scan: %v", err))
	} else {
		result.Agents = append(result.Agents, fileAgents...)
	}

	// Phase 3: IDE extension scanning
	ideAgents, err := e.ideScanner.Scan(e.rules.Agents)
	if err != nil {
		errors = append(errors, fmt.Sprintf("ide scan: %v", err))
	} else {
		result.Agents = append(result.Agents, ideAgents...)
	}

	// Phase 3.5: Extract version from IDE extensions
	for i := range result.Agents {
		agent := &result.Agents[i]
		if agent.Version == "" && agent.Extension != nil && agent.Extension.Version != "" {
			agent.Version = agent.Extension.Version
		}
	}

	// Phase 4: Config extraction for each agent
	for i := range result.Agents {
		agent := &result.Agents[i]
		e.extractAgentConfigs(agent)
	}

	// Phase 5: Skill discovery — runs independently for every agent
	// rule with skills.enabled: true. Skills are attached to discovered
	// agents, or a new ghost agent entry is created if skills were found
	// but the agent was not otherwise detected.
	for _, r := range e.rules.Agents {
		if r.Skills == nil || !r.Skills.Enabled {
			continue
		}

		// Collect file-evidence directories from already-discovered
		// instances of this agent to seed auto-probe
		var fileDirs []string
		for i := range result.Agents {
			if result.Agents[i].Name == r.Name {
				a := &result.Agents[i]
				for _, f := range a.Files {
					fileDirs = append(fileDirs, f.Path)
				}
				if a.Process != nil && a.Process.CWD != "" {
					fileDirs = append(fileDirs, a.Process.CWD)
				}
			}
		}

		var skillDir string
		skills, err := e.skillDiscover.DiscoverSkillsWithProbe(r, fileDirs, &skillDir)
		if err != nil {
			errors = append(errors, fmt.Sprintf("skill discovery for %s: %v", r.Name, err))
			continue
		}

		if len(skills) > 0 || skillDir != "" {
			// Attach skills to already-discovered instances
			found := false
			for i := range result.Agents {
				if result.Agents[i].Name == r.Name {
					found = true
					result.Agents[i].Skills = append(result.Agents[i].Skills, skills...)
					if skillDir != "" && result.Agents[i].SkillDir == "" {
						result.Agents[i].SkillDir = skillDir
					}
				}
			}
			// If no agent was discovered by other means, create a
			// ghost entry anchored by skill discovery
			if !found && len(skills) > 0 {
				agent := model.DiscoveredAgent{
					Name:        r.Name,
					DisplayName: r.DisplayName,
					Confidence:  model.Confidence(r.MinConfidence),
					AssetType:   model.AssetTypeFile, // skill files are file-system evidence
					Skills:      skills,
					SkillDir:    skillDir,
				}
				result.Agents = append(result.Agents, agent)
			}
		}
	}

	// Phase 6: Package manager scanning (npm, pip, apt, brew, cargo, gem, etc.)
	pkgAgents := e.pkgScanner.Scan(e.rules.Agents)
	result.Agents = append(result.Agents, pkgAgents...)

	// Phase 7: Binary-in-PATH scanning (which <name>, version extraction)
	binaryAgents := e.binaryScanner.Scan(e.rules.Agents)
	result.Agents = append(result.Agents, binaryAgents...)

	// Phase 8: Command-based probing (type + version via execution)
	probeAgents := e.probeScanner.Scan(e.rules.Agents)
	result.Agents = append(result.Agents, probeAgents...)

	// Deduplicate within each type first (e.g. two "possible" process entries)
	result.Agents = deduplicateAgents(result.Agents)

	// Cross-type merge: consolidate multiple evidence types for the same
	// agent name into a single entry.  When process evidence confirms the
	// agent, all other evidence types (file, binary, probe, package) are
	// merged as supporting signals and the overall confidence is promoted.
	result.Agents = mergeCrossType(result.Agents)

	// Populate summary
	e.populateSummary(result, errors)

	return result, nil
}

func (e *Engine) extractAgentConfigs(agent *model.DiscoveredAgent) {
	for _, r := range e.rules.Agents {
		if r.Name != agent.Name || r.Config == nil {
			continue
		}
		cfgRule := r.Config
		configData := make(map[string]any)

		for _, cfgPath := range cfgRule.Paths {
			expandedPath, err := config.ExpandPath(cfgPath)
			if err != nil {
				continue
			}
			data, err := os.ReadFile(expandedPath)
			if err != nil {
				continue
			}
			parsed := parseConfigFormat(data, cfgRule.Format)
			for targetKey, sourcePath := range cfgRule.FieldMap {
				if val := getNestedValue(parsed, sourcePath); val != nil {
					configData[targetKey] = val
				}
			}
		}
		if len(configData) > 0 {
			if agent.Config == nil {
				agent.Config = make(map[string]any)
			}
			for k, v := range configData {
				agent.Config[k] = v
			}
			if agent.Version == "" {
				if ver, ok := configData["version"]; ok {
					agent.Version = fmt.Sprintf("%v", ver)
				}
			}
		}
	}
}

func parseConfigFormat(data []byte, format string) map[string]any {
	var parsed map[string]any
	switch format {
	case "json":
		json.Unmarshal(data, &parsed)
	case "yaml", "yml":
		yaml.Unmarshal(data, &parsed)
	case "toml":
		parsed = parseSimpleTOML(string(data))
	case "env":
		parsed = parseEnvContent(string(data))
	}
	return parsed
}

func (e *Engine) populateSummary(result *Result, errors []string) {
	summary := &result.Summary
	summary.Errors = errors

	for _, agent := range result.Agents {
		summary.TotalAgents++
		switch agent.Confidence {
		case "confirmed":
			summary.ConfirmedAgents++
		case "possible":
			summary.PossibleAgents++
		case "ghost":
			summary.GhostAgents++
		}
		summary.ByType[string(agent.AssetType)]++
		summary.TotalSkills += len(agent.Skills)
	}
}

// deduplicateAgents removes duplicate agents within the same name+type combination.
// It also deduplicates skills by file_path to prevent duplicates when skills
// are collected from multiple scan phases.
func deduplicateAgents(agents []model.DiscoveredAgent) []model.DiscoveredAgent {
	seen := make(map[string]*model.DiscoveredAgent)
	for i := range agents {
		a := &agents[i]
		key := a.Name + ":" + string(a.AssetType)
		if existing, ok := seen[key]; ok {
			if confidenceRank(a.Confidence) > confidenceRank(existing.Confidence) {
				a.Skills = append(existing.Skills, a.Skills...)
				a.Files = append(existing.Files, a.Files...)
				// Upgrade version if the better-confidence entry has one
				if existing.Version != "" && a.Version == "" {
					a.Version = existing.Version
				}
				// Merge probe info
				if a.Probe == nil && existing.Probe != nil {
					a.Probe = existing.Probe
				}
				// Merge binary info
				if a.Binary == nil && existing.Binary != nil {
					a.Binary = existing.Binary
				}
				// Merge package info
				if a.Package == nil && existing.Package != nil {
					a.Package = existing.Package
				}
				seen[key] = a
			} else {
				existing.Skills = append(existing.Skills, a.Skills...)
				existing.Files = append(existing.Files, a.Files...)
				// Merge probe info from lower confidence entry
				if existing.Probe == nil && a.Probe != nil {
					existing.Probe = a.Probe
				}
				if existing.Binary == nil && a.Binary != nil {
					existing.Binary = a.Binary
				}
				if existing.Package == nil && a.Package != nil {
					existing.Package = a.Package
				}
			}
		} else {
			seen[key] = a
		}
	}
	var result []model.DiscoveredAgent
	for _, v := range seen {
		v.Skills = deduplicateSkills(v.Skills)
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// deduplicateSkills removes skills with the same FilePath, keeping the first.
// Skills with empty FilePath are never deduplicated against each other.
func deduplicateSkills(skills []model.Skill) []model.Skill {
	if len(skills) <= 1 {
		return skills
	}
	seen := make(map[string]bool)
	var deduped []model.Skill
	for _, s := range skills {
		// Only deduplicate by FilePath when it's non-empty
		if s.FilePath != "" {
			if seen[s.FilePath] {
				continue
			}
			seen[s.FilePath] = true
		}
		deduped = append(deduped, s)
	}
	return deduped
}

func confidenceRank(c model.Confidence) int {
	switch c {
	case "confirmed":
		return 3
	case "possible":
		return 2
	case "ghost":
		return 1
	default:
		return 0
	}
}

// mergeCrossType consolidates multiple DiscoveredAgent entries that share
// the same name but were produced by different scanners (process, file,
// binary, probe, package).  The entry with the highest confidence wins,
// and evidence from remaining entries is merged as supporting signals.
//
// AssetType preference when all confidences are equal: process > probe >
// binary > package > file > ide_extension.
func mergeCrossType(agents []model.DiscoveredAgent) []model.DiscoveredAgent {
	seen := make(map[string]*model.DiscoveredAgent)
	for i := range agents {
		a := &agents[i]
		if existing, ok := seen[a.Name]; ok {
			mergeAgentEvidence(existing, a)
		} else {
			seen[a.Name] = a
		}
	}
	var result []model.DiscoveredAgent
	for _, v := range seen {
		v.Skills = deduplicateSkills(v.Skills)
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// mergeAgentEvidence merges supporting evidence from src into dst.
// dst is the primary entry; src provides additional signals.
func mergeAgentEvidence(dst, src *model.DiscoveredAgent) {
	// Promote confidence if src has higher rank
	if confidenceRank(src.Confidence) > confidenceRank(dst.Confidence) {
		// Swap roles: src becomes primary
		src.Skills = append(src.Skills, dst.Skills...)
		src.Files = append(src.Files, dst.Files...)
		if src.Version == "" && dst.Version != "" {
			src.Version = dst.Version
		}
		if src.Process == nil && dst.Process != nil {
			src.Process = dst.Process
		}
		if src.Probe == nil && dst.Probe != nil {
			src.Probe = dst.Probe
		}
		if src.Binary == nil && dst.Binary != nil {
			src.Binary = dst.Binary
		}
		if src.Package == nil && dst.Package != nil {
			src.Package = dst.Package
		}
		if src.Extension == nil && dst.Extension != nil {
			src.Extension = dst.Extension
		}
		if len(src.Config) == 0 && len(dst.Config) > 0 {
			src.Config = dst.Config
		}
		if src.SkillDir == "" && dst.SkillDir != "" {
			src.SkillDir = dst.SkillDir
		}
		*dst = *src
		return
	}

	// dst has higher (or equal) confidence — merge src evidence into dst
	dst.Skills = append(dst.Skills, src.Skills...)
	dst.Files = append(dst.Files, src.Files...)
	if dst.Version == "" && src.Version != "" {
		dst.Version = src.Version
	}
	if dst.Process == nil && src.Process != nil {
		dst.Process = src.Process
	}
	if dst.Probe == nil && src.Probe != nil {
		dst.Probe = src.Probe
	}
	if dst.Binary == nil && src.Binary != nil {
		dst.Binary = src.Binary
	}
	if dst.Package == nil && src.Package != nil {
		dst.Package = src.Package
	}
	if dst.Extension == nil && src.Extension != nil {
		dst.Extension = src.Extension
	}
	if len(dst.Config) == 0 && len(src.Config) > 0 {
		dst.Config = src.Config
	}
	if dst.SkillDir == "" && src.SkillDir != "" {
		dst.SkillDir = src.SkillDir
	}

	// When confidence is equal, prefer an asset type with more evidence
	if dst.Confidence == src.Confidence {
		dst.AssetType = preferAssetType(dst.AssetType, src.AssetType)
	}
}

// preferAssetType returns the preferred asset type when confidence is equal.
func preferAssetType(a, b model.AssetType) model.AssetType {
	rank := map[model.AssetType]int{
		model.AssetTypeProcess:      6,
		model.AssetTypeProbe:        5,
		model.AssetTypeBinary:       4,
		model.AssetTypePackage:      3,
		model.AssetTypeFile:         2,
		model.AssetTypeIDEExtension: 1,
		model.AssetTypeConfig:       0,
	}
	if rank[a] >= rank[b] {
		return a
	}
	return b
}

func getNestedValue(data map[string]any, path string) any {
	if data == nil {
		return nil
	}
	keys := strings.Split(path, ".")
	var current any = data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func parseSimpleTOML(content string) map[string]any {
	result := make(map[string]any)
	currentSection := result
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := line[1 : len(line)-1]
			section := make(map[string]any)
			result[sectionName] = section
			currentSection = section
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		currentSection[key] = val
	}
	return result
}

func parseEnvContent(content string) map[string]any {
	result := make(map[string]any)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}
