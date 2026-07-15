package ide

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/model"
	"github.com/ai-asset-discovery/internal/platform"
)

// Scanner scans IDE extensions for AI agents.
// The scanner is IDE-agnostic: all IDE-specific knowledge (manifest file name,
// matching fields, agent directory probes) comes from the rule YAML, not from Go.
type Scanner struct {
	// manifestCache holds parsed manifest maps keyed by ExtPath,
	// used internally for matching/checking without leaking into output.
	manifestCache map[string]map[string]any
}

// NewScanner creates a new IDE scanner.
func NewScanner() *Scanner {
	return &Scanner{
		manifestCache: make(map[string]map[string]any),
	}
}

// Scan scans IDE extensions and returns matched agents.
func (s *Scanner) Scan(rules []model.AgentRule) ([]model.DiscoveredAgent, error) {
	var results []model.DiscoveredAgent

	for _, rule := range rules {
		if rule.IDE == nil {
			continue
		}
		matched := s.scanIDERule(rule)
		results = append(results, matched...)
	}
	return results, nil
}

// resolveScanPaths builds the list of extension-scan root directories from
// a rule. Only ScanPaths entries are used (no hardcoded platform presets).
func (s *Scanner) resolveScanPaths(ideRule *model.IDERule) []model.IDEScanPath {
	var out []model.IDEScanPath
	for _, sp := range ideRule.ScanPaths {
		if sp.Path != "" {
			out = append(out, sp)
		}
	}
	return out
}

func (s *Scanner) scanIDERule(rule model.AgentRule) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent
	ideRule := rule.IDE

	// Resolve scan paths: rule-driven IDEScanPath entries
	scanPaths := s.resolveScanPaths(ideRule)

	for _, sp := range scanPaths {
		// Skip OS-scoped paths that don't match the current platform.
		if sp.OS != "" && !strings.EqualFold(sp.OS, platform.CurrentOS()) {
			continue
		}

		expandedPath, err := config.ExpandPath(sp.Path)
		if err != nil {
			continue
		}

		extensions, err := s.scanExtensionsDir(expandedPath, ideRule)
		if err != nil {
			continue
		}

		for _, ext := range extensions {
			matched := s.matchExtension(ext, ideRule)
			if !matched {
				continue
			}

			// Clone the extension to avoid mutating shared state
			extCopy := *ext
			hasAgent := s.checkAgentCapability(&extCopy, ideRule)

			agent := model.DiscoveredAgent{
				Name:        rule.Name,
				DisplayName: rule.DisplayName,
				Confidence:  model.Confidence(rule.MinConfidence),
				AssetType:   model.AssetTypeIDEExtension,
				Extension:   &extCopy,
			}

			if hasAgent {
				agent.Confidence = "confirmed"
			}

			// Extract config
			if len(ideRule.ConfigKeys) > 0 && extCopy.Config == nil {
				extCopy.Config = make(map[string]any)
			}
			for _, ck := range ideRule.ConfigKeys {
				if val := s.extractConfigValue(&extCopy, ck.KeyPath); val != "" {
					extCopy.Config[ck.Field] = val
				}
			}

			results = append(results, agent)
		}
	}
	return results
}

// matchExtension checks whether an extension matches the rule's criteria.
func (s *Scanner) matchExtension(ext *model.IDEExtension, rule *model.IDERule) bool {
	hasExtIDs := len(rule.ExtIDs) > 0
	hasKeywords := len(rule.Keywords) > 0
	hasDeps := len(rule.Depends) > 0

	// Priority 1: Exact extension ID matching — when ExtIDs are specified,
	// ONLY match by ExtIDs (no keyword fallback, to prevent cross-contamination
	// where e.g. "github-copilot" rule's keyword "AI" matches Continue or Cline).
	if hasExtIDs {
		for _, id := range rule.ExtIDs {
			if strings.EqualFold(ext.ID, id) || matchGlob(ext.ID, id) {
				ext.IsAI = true
				return true
			}
		}
		return false
	}

	// Priority 2: Keyword / dependency heuristics (only when NO ExtIDs are specified)
	if hasKeywords || hasDeps {
		return s.isAIExtensionByHeuristics(ext, rule)
	}

	return false
}

// isAIExtensionByHeuristics tries keyword, dependency, and display-name matching.
// This is only called when ExtIDs are not set, or as a fallback.
func (s *Scanner) isAIExtensionByHeuristics(ext *model.IDEExtension, rule *model.IDERule) bool {
	manifest := s.getCachedManifest(ext)
	if manifest == nil {
		return false
	}

	// Check keywords against categories, extension keywords, and display name
	for _, kw := range rule.Keywords {
		kwLower := strings.ToLower(kw)
		// categories
		if cats, ok := manifest["categories"].([]any); ok {
			for _, c := range cats {
				if cs, ok := c.(string); ok && strings.Contains(strings.ToLower(cs), kwLower) {
					ext.IsAI = true
					return true
				}
			}
		}
		// keywords
		if kws, ok := manifest["keywords"].([]any); ok {
			for _, k := range kws {
				if ks, ok := k.(string); ok && strings.Contains(strings.ToLower(ks), kwLower) {
					ext.IsAI = true
					return true
				}
			}
		}
		// displayName
		if dn, ok := manifest["displayName"].(string); ok && strings.Contains(strings.ToLower(dn), kwLower) {
			ext.IsAI = true
			return true
		}
	}

	// Check dependencies
	for _, dep := range rule.Depends {
		depLower := strings.ToLower(dep)
		if deps, ok := manifest["dependencies"].(map[string]any); ok {
			for depName := range deps {
				if strings.Contains(strings.ToLower(depName), depLower) {
					ext.IsAI = true
					return true
				}
			}
		}
	}

	return false
}

func (s *Scanner) scanExtensionsDir(extDir string, ideRule *model.IDERule) ([]*model.IDEExtension, error) {
	entries, err := os.ReadDir(extDir)
	if err != nil {
		return nil, err
	}

	manifestFile := ideRule.ManifestFile
	if manifestFile == "" {
		manifestFile = "package.json"
	}

	var extensions []*model.IDEExtension
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgPath := filepath.Join(extDir, e.Name(), manifestFile)
		manifest, err := s.readManifest(pkgPath)
		if err != nil {
			continue
		}

		ext := buildExtension(manifest, extDir, e.Name())
		// Cache manifest in-memory for subsequent checks — does NOT leak into JSON.
		s.manifestCache[ext.ExtPath] = manifest
		extensions = append(extensions, ext)
	}
	return extensions, nil
}

// buildExtension populates an IDEExtension from a generic manifest map.
func buildExtension(manifest map[string]any, extDir, subDir string) *model.IDEExtension {
	publisher, _ := manifest["publisher"].(string)
	name, _ := manifest["name"].(string)
	displayName, _ := manifest["displayName"].(string)
	version, _ := manifest["version"].(string)
	description, _ := manifest["description"].(string)

	ext := &model.IDEExtension{
		ID:          fmt.Sprintf("%s.%s", publisher, name),
		Name:        displayName,
		Version:     version,
		Publisher:   publisher,
		Description: description,
		IDEPath:     extDir,
		ExtPath:     filepath.Join(extDir, subDir),
	}
	if ext.Name == "" {
		ext.Name = name
	}
	return ext
}

// getCachedManifest returns the manifest cached during scanExtensionsDir,
// or reads it from disk as fallback.
func (s *Scanner) getCachedManifest(ext *model.IDEExtension) map[string]any {
	if m, ok := s.manifestCache[ext.ExtPath]; ok && m != nil {
		return m
	}
	// Fallback: read from disk (use default manifest file name)
	pkgPath := filepath.Join(ext.ExtPath, "package.json")
	m, err := s.readManifest(pkgPath)
	if err != nil {
		return nil
	}
	s.manifestCache[ext.ExtPath] = m
	return m
}

func (s *Scanner) readManifest(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Scanner) checkAgentCapability(ext *model.IDEExtension, rule *model.IDERule) bool {
	var signals []string

	manifest := s.getCachedManifest(ext)
	if manifest == nil {
		return false
	}

	// Check activationEvents for "agent"
	if aeList, ok := manifest["activationEvents"].([]any); ok {
		for _, ae := range aeList {
			if a, ok := ae.(string); ok && strings.Contains(strings.ToLower(a), "agent") {
				signals = append(signals, "activation:agent")
				break
			}
		}
	}

	// Check contributes.agent
	if contributes, ok := manifest["contributes"].(map[string]any); ok {
		if _, ok := contributes["agent"]; ok {
			signals = append(signals, "contributes:agent")
		}
	}

	// Check main entry for agent export
	if main, ok := manifest["main"].(string); ok && strings.Contains(strings.ToLower(main), "agent") {
		signals = append(signals, "main:agent")
	}

	// Check for agent-specific directories — rule-driven
	agentDirs := rule.AgentDirs
	if len(agentDirs) == 0 {
		agentDirs = []string{"dist/agent", "out/agent", "skills", "tools"}
	}
	for _, dir := range agentDirs {
		checkPath := filepath.Join(ext.ExtPath, dir)
		if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
			signals = append(signals, "dir:"+dir)
		}
	}

	// Check for agent signals from rule inside main source file
	if main, ok := manifest["main"].(string); ok && main != "" {
		mainPath := filepath.Join(ext.ExtPath, main)
		if data, err := os.ReadFile(mainPath); err == nil {
			content := string(data)
			if len(content) > 102400 {
				content = content[:102400]
			}
			for _, sig := range rule.AgentSignals {
				if strings.Contains(content, sig) {
					signals = append(signals, "code:"+sig)
				}
			}
		}
	}

	ext.AgentSignals = signals
	ext.HasAgent = len(signals) > 0
	return ext.HasAgent
}

func (s *Scanner) extractConfigValue(ext *model.IDEExtension, keyPath string) string {
	manifest := s.getCachedManifest(ext)
	if manifest == nil {
		return ""
	}

	keys := strings.Split(keyPath, ".")
	current := any(manifest)
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
		if current == nil {
			return ""
		}
	}
	if str, ok := current.(string); ok {
		return str
	}
	return ""
}

func matchGlob(input, pattern string) bool {
	// Simple wildcard matching: * matches anything
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return strings.EqualFold(input, pattern)
	}

	remaining := input
	for i, part := range parts {
		if part == "" {
			if i == len(parts)-1 {
				return true
			}
			continue
		}
		idx := strings.Index(strings.ToLower(remaining), strings.ToLower(part))
		if idx < 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
		if i == 0 && idx != 0 {
			return false
		}
	}
	return len(parts[len(parts)-1]) == 0 || strings.HasSuffix(strings.ToLower(input), strings.ToLower(parts[len(parts)-1]))
}
