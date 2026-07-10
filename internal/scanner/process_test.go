package scanner

import (
	"os"
	"strings"
	"testing"

	"github.com/ai-asset-discovery/internal/model"
)

func TestProcessScanner_MatchPattern(t *testing.T) {
	ps := NewProcessScanner()

	tests := []struct {
		name    string
		pattern model.PatternRule
		value   string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: model.PatternRule{Type: "exact", Value: "claude", Weight: 10},
			value:   "claude",
			want:    true,
		},
		{
			name:    "exact no match",
			pattern: model.PatternRule{Type: "exact", Value: "claude", Weight: 10},
			value:   "claude-desktop",
			want:    false,
		},
		{
			name:    "contains match",
			pattern: model.PatternRule{Type: "contains", Value: "claude", Weight: 8},
			value:   "/usr/bin/claude-desktop",
			want:    true,
		},
		{
			name:    "contains no match",
			pattern: model.PatternRule{Type: "contains", Value: "claude", Weight: 8},
			value:   "chatgpt",
			want:    false,
		},
		{
			name:    "regex match",
			pattern: model.PatternRule{Type: "regex", Value: "python.*aider", Weight: 5},
			value:   "python -m aider --model gpt-4",
			want:    true,
		},
		{
			name:    "regex no match",
			pattern: model.PatternRule{Type: "regex", Value: "python.*aider", Weight: 5},
			value:   "node server.js",
			want:    false,
		},
		{
			name:    "default (contains)",
			pattern: model.PatternRule{Type: "", Value: "test", Weight: 1},
			value:   "this is a test",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ps.matchPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessScanner_EvaluateProcess_Or(t *testing.T) {
	ps := NewProcessScanner()

	pr := &model.ProcessRule{
		MatchLogic: "or",
		NamePatterns: []model.PatternRule{
			{Type: "exact", Value: "claude", Weight: 10},
		},
		CmdPatterns: []model.PatternRule{
			{Type: "contains", Value: "aider", Weight: 8},
		},
	}

	// Should match via cmdline
	proc := model.ProcessInfo{
		PID:     1234,
		Name:    "python",
		CmdLine: "python -m aider --model gpt-4",
	}

	matches := ps.evaluateProcess(pr, proc)
	if len(matches) == 0 {
		t.Error("expected match via cmdline (or logic)")
	}

	// Should match via name
	proc2 := model.ProcessInfo{
		PID:     5678,
		Name:    "claude",
		CmdLine: "/usr/bin/claude",
	}

	matches2 := ps.evaluateProcess(pr, proc2)
	if len(matches2) == 0 {
		t.Error("expected match via name (or logic)")
	}

	// Should NOT match
	proc3 := model.ProcessInfo{
		PID:     9999,
		Name:    "bash",
		CmdLine: "/bin/bash",
	}

	matches3 := ps.evaluateProcess(pr, proc3)
	if len(matches3) > 0 {
		t.Error("expected no match")
	}
}

func TestProcessScanner_EvaluateProcess_And(t *testing.T) {
	ps := NewProcessScanner()

	pr := &model.ProcessRule{
		MatchLogic: "and",
		NamePatterns: []model.PatternRule{
			{Type: "exact", Value: "claude", Weight: 10},
		},
		CmdPatterns: []model.PatternRule{
			{Type: "contains", Value: "--serve", Weight: 8},
		},
	}

	// Both match -> should pass
	proc := model.ProcessInfo{
		PID:     1234,
		Name:    "claude",
		CmdLine: "claude --serve --port 8080",
	}
	matches := ps.evaluateProcess(pr, proc)
	if len(matches) == 0 {
		t.Error("expected match when both name and cmd match")
	}

	// Only name matches -> should fail
	proc2 := model.ProcessInfo{
		PID:     5678,
		Name:    "claude",
		CmdLine: "claude --help",
	}
	matches2 := ps.evaluateProcess(pr, proc2)
	if len(matches2) > 0 {
		t.Error("expected no match when only name matches (and logic)")
	}
}

func TestProcessScanner_ExtractVersion(t *testing.T) {
	ps := NewProcessScanner()

	proc := model.ProcessInfo{
		Name:    "aider",
		CmdLine: "aider v0.50.1 --model gpt-4",
	}

	ver := ps.extractVersion(proc, `aider.*v?([0-9]+\.[0-9]+\.[0-9]+)`)
	if ver != "0.50.1" {
		t.Errorf("extractVersion() = %q, want %q", ver, "0.50.1")
	}

	ver2 := ps.extractVersion(proc, `nomatch`)
	if ver2 != "" {
		t.Errorf("extractVersion() = %q, want empty", ver2)
	}
}

func TestProcessScanner_ListProcesses(t *testing.T) {
	ps := NewProcessScanner()
	procs, err := ps.listProcesses()
	if err != nil {
		// On systems without /proc (macOS), this is expected to fail
		if _, statErr := os.Stat("/proc"); statErr != nil {
			t.Skipf("/proc not available: %v", err)
		}
		t.Fatalf("listProcesses() error: %v", err)
	}

	if len(procs) == 0 {
		t.Error("expected at least some processes")
	}

	// Check that we got the current process (this test)
	found := false
	for _, p := range procs {
		if p.PID == os.Getpid() {
			found = true
			if p.Name == "" {
				t.Error("process name should not be empty")
			}
			break
		}
	}
	if !found {
		t.Error("current process not found in process list")
	}
}

func TestProcessScanner_Scan(t *testing.T) {
	// This test uses the actual running process list, so we check that
	// no rules with impossible patterns match unexpectedly.
	ps := NewProcessScanner()

	rules := []model.AgentRule{
		{
			Name: "nothing-matches",
			Process: &model.ProcessRule{
				MatchLogic: "and",
				NamePatterns: []model.PatternRule{
					{Type: "exact", Value: "this-process-does-not-exist-xyz123", Weight: 10},
				},
			},
		},
	}

	results, err := ps.Scan(rules)
	if err != nil {
		if _, statErr := os.Stat("/proc"); statErr != nil {
			t.Skipf("/proc not available: %v", err)
		}
		t.Fatalf("Scan() error: %v", err)
	}

	if len(results) > 0 {
		t.Errorf("expected no matches, got %d: %v", len(results), results)
	}
}

// =========================================================================
// End-to-End Process Scanner Tests (scanner package — can access unexported)
// =========================================================================

// TestE2E_ConfidenceBumpGhostToPossible tests that ghost-level agents
// get bumped to "possible" when ≥2 patterns match.
func TestE2E_ConfidenceBumpGhostToPossible(t *testing.T) {
	ps := NewProcessScanner()

	rule := model.AgentRule{
		Name:          "ghost-agent",
		MinConfidence: "ghost",
		Process: &model.ProcessRule{
			MatchLogic: "or",
			NamePatterns: []model.PatternRule{
				{Type: "contains", Value: "ghost", Weight: 5},
			},
			CmdPatterns: []model.PatternRule{
				{Type: "contains", Value: "ghost-arg", Weight: 3},
			},
		},
	}

	// Process that matches BOTH name and cmd patterns
	procs := []model.ProcessInfo{
		{
			PID:     1234,
			Name:    "ghost-process",
			CmdLine: "ghost-process --ghost-arg --more",
		},
	}

	results := ps.matchProcesses(rule, procs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Confidence != model.ConfidencePossible {
		t.Errorf("Confidence = %q, want possible (bumped from ghost with ≥2 matches)",
			results[0].Confidence)
	}
}

// TestE2E_ConfidenceBumpToConfirmed tests that non-ghost rules with
// ≥2 pattern matches get bumped to "confirmed".
func TestE2E_ConfidenceBumpToConfirmed(t *testing.T) {
	ps := NewProcessScanner()

	rule := model.AgentRule{
		Name:          "test-agent",
		MinConfidence: "possible",
		Process: &model.ProcessRule{
			MatchLogic: "or",
			NamePatterns: []model.PatternRule{
				{Type: "contains", Value: "myagent", Weight: 10},
			},
			CmdPatterns: []model.PatternRule{
				{Type: "contains", Value: "--server", Weight: 8},
			},
			DirPatterns: []model.PatternRule{
				{Type: "contains", Value: "/opt/myagent", Weight: 5},
			},
		},
	}

	procs := []model.ProcessInfo{
		{
			PID:     1234,
			Name:    "myagent",
			CmdLine: "myagent --server --port 8080",
			CWD:     "/opt/myagent",
		},
	}

	results := ps.matchProcesses(rule, procs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Confidence != model.ConfidenceConfirmed {
		t.Errorf("Confidence = %q, want confirmed (≥2 matches with non-ghost base)",
			results[0].Confidence)
	}
}

// TestE2E_VersionExtraction verifies version regex extraction works.
func TestE2E_VersionExtraction(t *testing.T) {
	ps := NewProcessScanner()

	rule := model.AgentRule{
		Name:          "versioned-agent",
		MinConfidence: "possible",
		Process: &model.ProcessRule{
			MatchLogic:   "or",
			VersionRegex: `versioned-agent.*v?([0-9]+\.[0-9]+\.[0-9]+)`,
			CmdPatterns: []model.PatternRule{
				{Type: "contains", Value: "versioned-agent", Weight: 10},
			},
		},
	}

	procs := []model.ProcessInfo{
		{
			PID:     1234,
			Name:    "versioned-agent",
			CmdLine: "versioned-agent v2.3.4 --model gpt-4",
		},
	}

	results := ps.matchProcesses(rule, procs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Version != "2.3.4" {
		t.Errorf("Version = %q, want 2.3.4", results[0].Version)
	}
}

// TestE2E_MatchLogicAndAllFields tests AND logic with all fields matching.
func TestE2E_MatchLogicAndAllFields(t *testing.T) {
	ps := NewProcessScanner()

	pr := &model.ProcessRule{
		MatchLogic: "and",
		NamePatterns: []model.PatternRule{
			{Type: "exact", Value: "myapp", Weight: 10},
		},
		CmdPatterns: []model.PatternRule{
			{Type: "contains", Value: "--serve", Weight: 8},
		},
		ExePatterns: []model.PatternRule{
			{Type: "contains", Value: "/usr/bin/myapp", Weight: 5},
		},
		DirPatterns: []model.PatternRule{
			{Type: "contains", Value: "/opt/myapp", Weight: 5},
		},
	}

	// All fields match
	proc := model.ProcessInfo{
		PID:        1234,
		Name:       "myapp",
		CmdLine:    "myapp --serve --port 8080",
		CWD:        "/opt/myapp/workspace",
		Executable: "/usr/bin/myapp",
	}
	matches := ps.evaluateProcess(pr, proc)
	if len(matches) == 0 {
		t.Error("expected match when ALL fields match (and logic)")
	}

	// Only two fields match -> should fail
	proc2 := model.ProcessInfo{
		PID:        5678,
		Name:       "myapp",
		CmdLine:    "myapp --serve",
		CWD:        "/tmp/elsewhere",
		Executable: "/usr/bin/other",
	}
	matches2 := ps.evaluateProcess(pr, proc2)
	if len(matches2) > 0 {
		t.Error("expected no match when not all fields match (and logic)")
	}
}

// TestE2E_MatchLogicAndEmptyGroups tests AND logic where empty groups
// default to "hit" (so only non-empty groups need to match).
func TestE2E_MatchLogicAndEmptyGroups(t *testing.T) {
	ps := NewProcessScanner()

	pr := &model.ProcessRule{
		MatchLogic: "and",
		NamePatterns: []model.PatternRule{
			{Type: "contains", Value: "myapp", Weight: 10},
		},
		CmdPatterns: []model.PatternRule{
			{Type: "contains", Value: "--serve", Weight: 8},
		},
		// ExePatterns and DirPatterns are EMPTY — should default to "hit" (true)
	}

	// Name and cmd match, exe/dir are empty -> should pass AND
	proc := model.ProcessInfo{
		PID:     1234,
		Name:    "myapp",
		CmdLine: "myapp --serve",
	}
	matches := ps.evaluateProcess(pr, proc)
	if len(matches) == 0 {
		t.Error("expected match when name+cmd match and exe/dir groups are empty (and)")
	}
}

// TestE2E_CrossPlatformEmptyFields tests that processes with empty
// Executable/CWD/User (as on macOS/Windows) still match correctly.
func TestE2E_CrossPlatformEmptyFields(t *testing.T) {
	ps := NewProcessScanner()

	pr := &model.ProcessRule{
		MatchLogic: "and",
		NamePatterns: []model.PatternRule{
			{Type: "exact", Value: "test-agent", Weight: 10},
		},
		CmdPatterns: []model.PatternRule{
			{Type: "contains", Value: "test-arg", Weight: 8},
		},
	}

	// macOS/Windows: CWD and Executable are empty strings
	proc := model.ProcessInfo{
		PID:        12345,
		Name:       "test-agent",
		CmdLine:    "test-agent --test-arg --verbose",
		CWD:        "",
		Executable: "",
		User:       "",
	}
	matches := ps.evaluateProcess(pr, proc)
	if len(matches) == 0 {
		t.Error("expected match with empty CWD/Executable/User (cross-platform)")
	}
}

// TestE2E_MultipleProcessMatchesSameRule tests that when multiple
// processes match the same rule, all are returned.
func TestE2E_MultipleProcessMatchesSameRule(t *testing.T) {
	ps := NewProcessScanner()

	rule := model.AgentRule{
		Name:          "multi-instance",
		MinConfidence: "possible",
		Process: &model.ProcessRule{
			MatchLogic: "and",
			NamePatterns: []model.PatternRule{
				{Type: "exact", Value: "node", Weight: 5},
			},
			CmdPatterns: []model.PatternRule{
				{Type: "contains", Value: "ai-worker", Weight: 10},
			},
		},
	}

	// Multiple matching processes
	procs := []model.ProcessInfo{
		{PID: 1001, Name: "node", CmdLine: "node ai-worker --port 3001"},
		{PID: 1002, Name: "node", CmdLine: "node ai-worker --port 3002"},
		{PID: 1003, Name: "node", CmdLine: "node regular-server"},
	}

	results := ps.matchProcesses(rule, procs)
	// First two match (name=node AND cmd contains ai-worker); third fails AND
	if len(results) != 2 {
		t.Fatalf("expected 2 matching processes, got %d", len(results))
	}
	for _, a := range results {
		if a.Name != "multi-instance" {
			t.Errorf("Name = %q, want multi-instance", a.Name)
		}
		if a.Process == nil {
			t.Fatal("Process is nil")
		}
		if !strings.Contains(a.Process.CmdLine, "ai-worker") {
			t.Errorf("Process %d should contain ai-worker in cmdline", a.Process.PID)
		}
	}
}

// TestE2E_SelfProcessExclusion verifies that matchProcesses has no
// PID-based filtering (that's done at the Scan level).
func TestE2E_SelfProcessExclusion(t *testing.T) {
	ps := NewProcessScanner()
	selfPID := os.Getpid()

	rule := model.AgentRule{
		Name:          "self-detect-test",
		MinConfidence: "possible",
		Process: &model.ProcessRule{
			MatchLogic: "or",
			NamePatterns: []model.PatternRule{
				{Type: "exact", Value: "discovery.test", Weight: 10},
			},
		},
	}

	procs := []model.ProcessInfo{
		{PID: selfPID, Name: "discovery.test", CmdLine: "discovery.test"},
		{PID: selfPID + 1, Name: "other", CmdLine: "other"},
	}

	// matchProcesses does NOT filter selfPID — that's done in Scan()
	// Only the first process should match (exact match on "discovery.test")
	results := ps.matchProcesses(rule, procs)
	if len(results) != 1 {
		t.Fatalf("matchProcesses should match 1 (exact name), got %d", len(results))
	}
	if results[0].Process != nil && results[0].Process.PID == selfPID {
		t.Logf("self PID %d was matched by matchProcesses (expected, Scan filters it)", selfPID)
	}
}
