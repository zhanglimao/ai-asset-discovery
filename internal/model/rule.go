package model

// AgentRule defines the detection pattern for a single AI agent.
type AgentRule struct {
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"display_name" json:"display_name"`
	Description string `yaml:"description" json:"description"`
	Version     string `yaml:"version" json:"version"`
	Category    string `yaml:"category" json:"category"`

	// ── Simplified feature-based detection (recommended) ──
	// Features describes WHAT to detect without dictating HOW.
	// The engine internally handles matching logic, weights, and
	// pattern types — rules only provide the fingerprints.
	Features *FeaturesRule `yaml:"features,omitempty" json:"features,omitempty"`

	// ── Simplified path detection ──
	// Paths is a flat list of file/directory paths that, when present,
	// indicate this agent is installed.
	Paths []PathRule `yaml:"paths,omitempty" json:"paths,omitempty"`

	// ── Command-based probing (type + version) ──
	// Probe defines commands to run that identify the agent type
	// and extract its version.
	Probe *ProbeRule `yaml:"probe,omitempty" json:"probe,omitempty"`

	// ── Legacy detailed fields (still supported; auto-populated
	//     from Features if present) ──
	Process *ProcessRule `yaml:"process,omitempty" json:"process,omitempty"`
	Files   []FileRule   `yaml:"files,omitempty" json:"files,omitempty"`
	IDE     *IDERule     `yaml:"ide,omitempty" json:"ide,omitempty"`
	Config  *ConfigRule  `yaml:"config,omitempty" json:"config,omitempty"`
	Skills  *SkillRule   `yaml:"skills,omitempty" json:"skills,omitempty"`
	Package *PackageRule `yaml:"package,omitempty" json:"package,omitempty"`
	Binary  *BinaryRule  `yaml:"binary,omitempty" json:"binary,omitempty"`

	// Minimum confidence for this rule to produce a result
	MinConfidence string `yaml:"min_confidence" json:"min_confidence"`
}

// ============================================================
// Simplified feature-based detection (recommended for rule authors)
// ============================================================

// FeaturesRule describes agent fingerprints using plain string lists.
// The engine internally handles matching — rules only say WHAT to look for.
type FeaturesRule struct {
	// Process names or cmdline substrings to match (case-insensitive contains).
	// Matches against both process name and full command line.
	Processes []string `yaml:"processes,omitempty" json:"processes,omitempty"`

	// Package names (npm/pip/apt/brew/etc.) to match.
	Packages []string `yaml:"packages,omitempty" json:"packages,omitempty"`

	// Binary names in $PATH to match.
	Binaries []string `yaml:"binaries,omitempty" json:"binaries,omitempty"`

	// IDE extension IDs to match (full or partial).
	Extensions []string `yaml:"extensions,omitempty" json:"extensions,omitempty"`

	// Agent capability keywords in extension manifests.
	AgentSignals []string `yaml:"agent_signals,omitempty" json:"agent_signals,omitempty"`

	// Version regex to extract from process cmdline or --version output.
	VersionRegex string `yaml:"version_regex,omitempty" json:"version_regex,omitempty"`

	// Binary version flag override (default: "--version").
	VersionFlag string `yaml:"version_flag,omitempty" json:"version_flag,omitempty"`
}

// ============================================================
// Command-based probing (type detection + version extraction)
// ============================================================

// ProbeRule defines a command to execute that identifies an agent's
// type and/or extracts its version at runtime.
type ProbeRule struct {
	// Command to execute (e.g. "claude", "aider", "gemini").
	// Must be in $PATH.
	Command string `yaml:"command" json:"command"`

	// Arguments to pass (e.g. ["--version"], ["-V"], ["version"]).
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Regex to extract the version from command output.
	// First capture group is the version.
	VersionRegex string `yaml:"version_regex,omitempty" json:"version_regex,omitempty"`

	// ExpectedOutput is an optional substring that must appear in
	// the command output to confirm this agent type.
	// If empty, any successful execution counts as a match.
	ExpectedOutput string `yaml:"expected_output,omitempty" json:"expected_output,omitempty"`
}

// ============================================================
// Simplified path detection
// ============================================================

// PathRule is a simpler alternative to FileRule for most use cases.
type PathRule struct {
	// Path to a file or directory (supports ~/, %APPDATA%, etc.).
	Path string `yaml:"path" json:"path"`

	// When true, this path must exist for the agent to be detected.
	// Default false (presence is optional evidence).
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`

	// OS filter: linux, darwin, windows, all (default: all).
	OS string `yaml:"os,omitempty" json:"os,omitempty"`
}

// ============================================================
// Legacy detailed rule types (for backward compatibility)
// ============================================================

// ProcessRule detects an agent by its running process.
type ProcessRule struct {
	NamePatterns []PatternRule `yaml:"name_patterns,omitempty" json:"name_patterns,omitempty"`
	CmdPatterns  []PatternRule `yaml:"cmd_patterns,omitempty" json:"cmd_patterns,omitempty"`
	ExePatterns  []PatternRule `yaml:"exe_patterns,omitempty" json:"exe_patterns,omitempty"`
	DirPatterns  []PatternRule `yaml:"dir_patterns,omitempty" json:"dir_patterns,omitempty"`
	MatchLogic   string        `yaml:"match_logic" json:"match_logic"`
	VersionRegex string        `yaml:"version_regex,omitempty" json:"version_regex,omitempty"`
}

// PatternRule defines a single matching pattern.
type PatternRule struct {
	Type   string `yaml:"type" json:"type"`
	Value  string `yaml:"value" json:"value"`
	Weight int    `yaml:"weight" json:"weight"`
}

// FileRule detects an agent by file/directory evidence.
type FileRule struct {
	Path     string `yaml:"path" json:"path"`
	FileType string `yaml:"file_type" json:"file_type"`
	Required bool   `yaml:"required" json:"required"`
	Contains string `yaml:"contains,omitempty" json:"contains,omitempty"`
	MaxDepth int    `yaml:"max_depth,omitempty" json:"max_depth,omitempty"`
	OS       string `yaml:"os,omitempty" json:"os,omitempty"`
}

// IDERule detects AI extensions in IDEs.
type IDERule struct {
	IDEType      string          `yaml:"ide_type,omitempty" json:"ide_type,omitempty"`
	Paths        []string        `yaml:"paths,omitempty" json:"paths,omitempty"`
	ExtIDs       []string        `yaml:"ext_ids,omitempty" json:"ext_ids,omitempty"`
	Keywords     []string        `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Depends      []string        `yaml:"depends,omitempty" json:"depends,omitempty"`
	AgentSignals []string        `yaml:"agent_signals,omitempty" json:"agent_signals,omitempty"`
	ConfigKeys   []ConfigExtract `yaml:"config_keys,omitempty" json:"config_keys,omitempty"`
}

// ConfigRule defines how to extract configuration.
type ConfigRule struct {
	Format   string            `yaml:"format" json:"format"`
	Paths    []string          `yaml:"paths" json:"paths"`
	FieldMap map[string]string `yaml:"field_map" json:"field_map"`
}

// ConfigExtract maps a config key path to a target field.
type ConfigExtract struct {
	Field   string `yaml:"field" json:"field"`
	KeyPath string `yaml:"key_path" json:"key_path"`
}

// SkillRule defines how to discover skills for an agent.
type SkillRule struct {
	// Enabled controls whether skill discovery is active for this agent.
	// When true, the engine proactively scans skill directories even if
	// the agent was not detected by other methods.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Paths to scan for skill files
	ScanPaths []string `yaml:"scan_paths" json:"scan_paths"`
	// File extensions to consider
	Extensions []string `yaml:"extensions" json:"extensions"`
	// Keywords that must appear in skill files
	Keywords []string `yaml:"keywords" json:"keywords"`
	// Max recursion depth
	MaxDepth int `yaml:"max_depth" json:"max_depth"`
	// Max file size to parse (KB)
	MaxSizeKB int `yaml:"max_size_kb" json:"max_size_kb"`
	// AutoDiscover enables automatic probing for skill directories under
	// file-evidence directories (e.g. ~/.cline → probes ~/.cline/skills,
	// ~/.cline/tools, ~/.cline/agents, etc.)
	// Default: true (enabled automatically when skills.enabled is true).
	// Set explicitly to false in YAML to disable auto-probing.
	AutoDiscover *bool `yaml:"auto_discover" json:"auto_discover"`
}

// RulesFile is the top-level structure of a rules YAML file.
type RulesFile struct {
	Version string      `yaml:"version" json:"version"`
	Agents  []AgentRule `yaml:"agents" json:"agents"`
}

// PackageRule detects an agent via installed packages (npm, pip, apt, etc.).
type PackageRule struct {
	Managers     []string         `yaml:"managers" json:"managers"`
	Packages     []PackagePattern `yaml:"packages" json:"packages"`
	VersionRegex string           `yaml:"version_regex,omitempty" json:"version_regex,omitempty"`
}

// PackagePattern defines a pattern to match a package name.
type PackagePattern struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// BinaryRule detects an agent by its CLI binary being present in PATH.
type BinaryRule struct {
	Names        []PatternRule `yaml:"names,omitempty" json:"names,omitempty"`
	VersionFlag  string        `yaml:"version_flag,omitempty" json:"version_flag,omitempty"`
	VersionRegex string        `yaml:"version_regex,omitempty" json:"version_regex,omitempty"`
}
