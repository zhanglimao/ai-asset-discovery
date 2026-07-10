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
type Scanner struct{}

// NewScanner creates a new IDE scanner.
func NewScanner() *Scanner {
	return &Scanner{}
}

// VSCodeExtensionManifest represents a VS Code extension's package.json.
type VSCodeExtensionManifest struct {
	Name             string            `json:"name"`
	DisplayName      string            `json:"displayName"`
	Version          string            `json:"version"`
	Publisher        string            `json:"publisher"`
	Description      string            `json:"description"`
	Categories       []string          `json:"categories"`
	Keywords         []string          `json:"keywords"`
	Main             string            `json:"main"`
	Contributes      map[string]any    `json:"contributes"`
	ActivationEvents []string          `json:"activationEvents"`
	Dependencies     map[string]string `json:"dependencies"`
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

// resolveScanPaths returns the list of extension directories to scan.
// When ide_type is set, it first tries platform-specific auto-discovery
// (e.g. %USERPROFILE%\.vscode\extensions on Windows), then falls back
// to the explicit Paths list. When ide_type is empty, only explicit
// Paths are used.
func (s *Scanner) resolveScanPaths(ideRule *model.IDERule) []string {
	var paths []string

	if ideRule.IDEType != "" {
		ide := platform.IDE(ideRule.IDEType)
		autoDirs := platform.ExtensionsDirs(ide)
		paths = append(paths, autoDirs...)
	}

	// Also append explicit paths as fallback
	paths = append(paths, ideRule.Paths...)

	return paths
}

func (s *Scanner) scanIDERule(rule model.AgentRule) []model.DiscoveredAgent {
	var results []model.DiscoveredAgent
	ideRule := rule.IDE

	// Resolve scan paths: auto-discovery via ide_type first,
	// then fall back to explicit rule paths.
	scanPaths := s.resolveScanPaths(ideRule)

	for _, basePath := range scanPaths {
		expandedPath, err := config.ExpandPath(basePath)
		if err != nil {
			continue
		}

		extensions, err := s.scanExtensionsDir(expandedPath)
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
	pkgPath := filepath.Join(ext.ExtPath, "package.json")
	manifest, err := s.readManifest(pkgPath)
	if err != nil {
		return false
	}

	// Check keywords against categories, extension keywords, and display name
	for _, kw := range rule.Keywords {
		kwLower := strings.ToLower(kw)
		for _, cat := range manifest.Categories {
			if strings.Contains(strings.ToLower(cat), kwLower) {
				ext.IsAI = true
				return true
			}
		}
		for _, ekw := range manifest.Keywords {
			if strings.Contains(strings.ToLower(ekw), kwLower) {
				ext.IsAI = true
				return true
			}
		}
		if strings.Contains(strings.ToLower(manifest.DisplayName), kwLower) {
			ext.IsAI = true
			return true
		}
	}

	// Check dependencies
	for _, dep := range rule.Depends {
		depLower := strings.ToLower(dep)
		for depName := range manifest.Dependencies {
			if strings.Contains(strings.ToLower(depName), depLower) {
				ext.IsAI = true
				return true
			}
		}
	}

	return false
}

func (s *Scanner) scanExtensionsDir(extDir string) ([]*model.IDEExtension, error) {
	entries, err := os.ReadDir(extDir)
	if err != nil {
		return nil, err
	}

	var extensions []*model.IDEExtension
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgPath := filepath.Join(extDir, e.Name(), "package.json")
		manifest, err := s.readManifest(pkgPath)
		if err != nil {
			continue
		}

		ext := &model.IDEExtension{
			ID:          fmt.Sprintf("%s.%s", manifest.Publisher, manifest.Name),
			Name:        manifest.DisplayName,
			Version:     manifest.Version,
			Publisher:   manifest.Publisher,
			Description: manifest.Description,
			IDEPath:     extDir,
			ExtPath:     filepath.Join(extDir, e.Name()),
		}
		if ext.Name == "" {
			ext.Name = manifest.Name
		}
		extensions = append(extensions, ext)
	}
	return extensions, nil
}

func (s *Scanner) readManifest(path string) (*VSCodeExtensionManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m VSCodeExtensionManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Scanner) checkAgentCapability(ext *model.IDEExtension, rule *model.IDERule) bool {
	var signals []string

	pkgPath := filepath.Join(ext.ExtPath, "package.json")
	manifest, err := s.readManifest(pkgPath)
	if err != nil {
		return false
	}

	// Check activationEvents for "agent"
	for _, ae := range manifest.ActivationEvents {
		if strings.Contains(strings.ToLower(ae), "agent") {
			signals = append(signals, "activation:agent")
		}
	}

	// Check contributes.agent
	if contributes, ok := manifest.Contributes["agent"]; ok && contributes != nil {
		signals = append(signals, "contributes:agent")
	}

	// Check main entry for agent export
	if strings.Contains(strings.ToLower(manifest.Main), "agent") {
		signals = append(signals, "main:agent")
	}

	// Check for agent-specific directories
	agentDirs := []string{"dist/agent", "out/agent", "skills", "tools", ".cline", ".continue"}
	for _, dir := range agentDirs {
		checkPath := filepath.Join(ext.ExtPath, dir)
		if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
			signals = append(signals, "dir:"+dir)
		}
	}

	// Check for agent signals from rule
	mainPath := filepath.Join(ext.ExtPath, manifest.Main)
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

	ext.AgentSignals = signals
	ext.HasAgent = len(signals) > 0
	return ext.HasAgent
}

func (s *Scanner) extractConfigValue(ext *model.IDEExtension, keyPath string) string {
	// Read contributes.configuration from package.json
	pkgPath := filepath.Join(ext.ExtPath, "package.json")
	manifest, err := s.readManifest(pkgPath)
	if err != nil {
		return ""
	}

	// Try to find nested key
	keys := strings.Split(keyPath, ".")
	current := any(manifest)
	for _, key := range keys {
		switch v := current.(type) {
		case map[string]any:
			current = v[key]
		case *VSCodeExtensionManifest:
			// Handle top-level known fields
			m := current.(*VSCodeExtensionManifest)
			switch key {
			case "version":
				return m.Version
			case "name":
				return m.Name
			case "displayName":
				return m.DisplayName
			case "publisher":
				return m.Publisher
			case "description":
				return m.Description
			default:
				return ""
			}
		default:
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
