// Package model defines the core data structures for AI asset discovery.
package model

// Confidence level for asset identification results.
type Confidence string

const (
	ConfidenceConfirmed Confidence = "confirmed"
	ConfidencePossible  Confidence = "possible"
	ConfidenceGhost     Confidence = "ghost"
)

// AssetType categorizes the discovered AI asset.
type AssetType string

const (
	AssetTypeProcess      AssetType = "process"
	AssetTypeFile         AssetType = "file"
	AssetTypeIDEExtension AssetType = "ide_extension"
	AssetTypeConfig       AssetType = "config"
	AssetTypePackage      AssetType = "package"
	AssetTypeBinary       AssetType = "binary"
	AssetTypeProbe        AssetType = "probe"
)

// ProcessInfo holds information about a running process.
type ProcessInfo struct {
	PID        int    `json:"pid"`
	Name       string `json:"name"`
	CmdLine    string `json:"cmdline"`
	CWD        string `json:"cwd"`
	Executable string `json:"executable"`
	PPID       int    `json:"ppid"`
	User       string `json:"user"`
}

// FileEvidence records a file or directory match.
type FileEvidence struct {
	Path       string `json:"path"`
	RuleSource string `json:"rule_source"` // which detection rule name
	MatchType  string `json:"match_type"`  // file, dir, content
	Content    string `json:"content,omitempty"`
}

// IDEExtension represents a discovered IDE extension.
type IDEExtension struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Publisher    string         `json:"publisher"`
	Description  string         `json:"description"`
	IsAI         bool           `json:"is_ai"`
	HasAgent     bool           `json:"has_agent"`
	IDEPath      string         `json:"ide_path"`
	ExtPath      string         `json:"ext_path"`
	Config       map[string]any `json:"config,omitempty"`
	AgentSignals []string       `json:"agent_signals,omitempty"`
}

// DiscoveredAgent is the final result of discovering one AI agent instance.
type DiscoveredAgent struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Version     string         `json:"version"`
	Confidence  Confidence     `json:"confidence"`
	AssetType   AssetType      `json:"asset_type"`
	Process     *ProcessInfo   `json:"process,omitempty"`
	Files       []FileEvidence `json:"files,omitempty"`
	Extension   *IDEExtension  `json:"extension,omitempty"`
	Package     *PackageInfo   `json:"package,omitempty"`
	Binary      *BinaryInfo    `json:"binary,omitempty"`
	Probe       *ProbeInfo     `json:"probe,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Skills      []Skill        `json:"skills,omitempty"`
	SkillDir    string         `json:"skill_dir,omitempty"` // root directory where skills were discovered
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Skill represents a discovered AI agent skill.
// Follows the Agent Skills specification: https://agentskills.io/specification
// Skills are directories containing a SKILL.md file with YAML frontmatter.
type Skill struct {
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name,omitempty"`
	Description     string         `json:"description"`
	Version         string         `json:"version,omitempty"`
	License         string         `json:"license,omitempty"`       // from Agent Skills spec
	Compatibility   string         `json:"compatibility,omitempty"` // from Agent Skills spec, e.g. ">=1.0.0"
	Tools           []SkillTool    `json:"tools,omitempty"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	PromptTemplate  string         `json:"prompt_template,omitempty"`
	TriggerPatterns []string       `json:"trigger_patterns,omitempty"`
	Dependencies    []string       `json:"dependencies,omitempty"`
	FilePath        string         `json:"file_path"`
	Format          string         `json:"format"` // markdown, yaml, json
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// SkillTool is a tool referenced by a Skill.
type SkillTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ============================================================
// Package-based detection
// ============================================================

// PackageInfo holds information about an installed software package.
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Manager string `json:"manager"` // npm, pip, apt, brew, cargo, gem, etc.
	Path    string `json:"path,omitempty"`
	Scope   string `json:"scope,omitempty"` // global, project, user
}

// BinaryInfo holds information about a discovered CLI binary in PATH.
type BinaryInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// ============================================================
// Command-based probing (type + version)
// ============================================================

// ProbeInfo holds the result of executing a probe command.
type ProbeInfo struct {
	Command string `json:"command"`
	Output  string `json:"output,omitempty"`
	Matched bool   `json:"matched"`
}
