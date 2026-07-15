package rule

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/ai-asset-discovery/internal/model"
)

// Loader reads agent detection rules from YAML files.
type Loader struct{}

// NewLoader creates a new Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// LoadFile loads rules from a single YAML file.
func (l *Loader) LoadFile(path string) (*model.RulesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules file %s: %w", path, err)
	}
	return l.Parse(data)
}

// LoadDir loads and merges all .yaml/.yml rule files in a directory.
func (l *Loader) LoadDir(dir string) (*model.RulesFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read rules dir %s: %w", dir, err)
	}

	merged := &model.RulesFile{Version: "1.0"}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !hasYamlExt(name) {
			continue
		}
		rf, err := l.LoadFile(dir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		merged.Agents = append(merged.Agents, rf.Agents...)
		// Merge package manager definitions
		if merged.PackageManagers == nil {
			merged.PackageManagers = make(map[string]model.PackageManagerDef)
		}
		for k, v := range rf.PackageManagers {
			merged.PackageManagers[k] = v
		}
	}
	return merged, nil
}

// Parse parses rules from YAML bytes.
func (l *Loader) Parse(data []byte) (*model.RulesFile, error) {
	var rf model.RulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse rules yaml: %w", err)
	}
	// Set defaults and normalize simplified syntax
	for i := range rf.Agents {
		if rf.Agents[i].MinConfidence == "" {
			rf.Agents[i].MinConfidence = "possible"
		}
		if rf.Agents[i].Process != nil && rf.Agents[i].Process.MatchLogic == "" {
			rf.Agents[i].Process.MatchLogic = "or"
		}
		if rf.Agents[i].Skills != nil {
			if rf.Agents[i].Skills.MaxDepth == 0 {
				rf.Agents[i].Skills.MaxDepth = 3
			}
			if rf.Agents[i].Skills.MaxSizeKB == 0 {
				rf.Agents[i].Skills.MaxSizeKB = 100
			}
			// auto_discover defaults to true
			if rf.Agents[i].Skills.AutoDiscover == nil {
				v := true
				rf.Agents[i].Skills.AutoDiscover = &v
			}
		}
		// Normalize simplified features → legacy fields
		l.normalizeFeatures(&rf.Agents[i])
		// Normalize simplified paths → legacy file rules
		l.normalizePaths(&rf.Agents[i])
	}
	return &rf, nil
}

// normalizeFeatures converts simplified Features into legacy scanner-specific
// fields (Process, Package, Binary, IDE) so existing scanners work unchanged.
func (l *Loader) normalizeFeatures(rule *model.AgentRule) {
	if rule.Features == nil {
		return
	}
	f := rule.Features

	// ── Process detection ──
	if len(f.Processes) > 0 {
		if rule.Process == nil {
			rule.Process = &model.ProcessRule{MatchLogic: "or"}
		}
		// Name patterns use word-boundary matching: a process named "omp"
		// must not match "WUDFCompanionHost" just because "Companion"
		// contains "omp" as a substring.
		// CmdLine patterns also use word-boundary by default to prevent
		// false positives on paths/arguments that contain the pattern
		// as a substring of a longer token (e.g. "openai.chatgpt-extension"
		// matching a "chatgpt" rule).
		for _, proc := range f.Processes {
			rule.Process.NamePatterns = append(rule.Process.NamePatterns, model.PatternRule{
				Type: "word", Value: proc, Weight: 5,
			})
			rule.Process.CmdPatterns = append(rule.Process.CmdPatterns, model.PatternRule{
				Type: "word", Value: proc, Weight: 8,
			})
		}
		// If features has a version_regex, forward it to process rule
		if f.VersionRegex != "" && rule.Process.VersionRegex == "" {
			rule.Process.VersionRegex = f.VersionRegex
		}
	}

	// ── Package detection ──
	if len(f.Packages) > 0 {
		if rule.Package == nil {
			// Default managers: query all common package managers.
			// The scanner skips unavailable ones at runtime, so it's
			// safe to include all. Rules can override by specifying
			// an explicit package.managers field.
			rule.Package = &model.PackageRule{
				Managers: []string{"npm", "pip", "pip3", "apt", "brew", "cargo", "gem"},
			}
		}
		for _, pkg := range f.Packages {
			rule.Package.Packages = append(rule.Package.Packages, model.PackagePattern{
				Name: pkg, Type: "exact",
			})
		}
	}

	// ── Binary detection ──
	if len(f.Binaries) > 0 {
		if rule.Binary == nil {
			rule.Binary = &model.BinaryRule{}
		}
		for _, bin := range f.Binaries {
			rule.Binary.Names = append(rule.Binary.Names, model.PatternRule{
				Type: "exact", Value: bin, Weight: 10,
			})
		}
		if f.VersionFlag != "" {
			rule.Binary.VersionFlag = f.VersionFlag
		}
		if f.VersionRegex != "" {
			rule.Binary.VersionRegex = f.VersionRegex
		}
	}

	// ── IDE extension detection ──
	// Features extensions / agent_signals are merged into the rule's IDE
	// section. If no IDE section exists yet we create one; the rule MUST
	// supply scan_paths to tell the scanner where to look.
	if len(f.Extensions) > 0 || len(f.AgentSignals) > 0 {
		if rule.IDE == nil {
			rule.IDE = &model.IDERule{}
		}
		rule.IDE.ExtIDs = append(rule.IDE.ExtIDs, f.Extensions...)
		rule.IDE.AgentSignals = append(rule.IDE.AgentSignals, f.AgentSignals...)
	}
}

// normalizePaths converts simplified flat Path rules into legacy FileRule
// entries so the existing FileScanner works unchanged.
func (l *Loader) normalizePaths(rule *model.AgentRule) {
	if len(rule.Paths) == 0 {
		return
	}
	for _, pr := range rule.Paths {
		rule.Files = append(rule.Files, model.FileRule{
			Path:     pr.Path,
			FileType: "directory",
			Required: pr.Required,
			OS:       pr.OS,
		})
	}
}

func hasYamlExt(name string) bool {
	return len(name) > 4 && (name[len(name)-5:] == ".yaml" || name[len(name)-4:] == ".yml")
}
