// Package scanner — Windows process listing via WMI (Get-CimInstance) with
// tasklist.exe fallback for systems where PowerShell is unavailable.
package scanner

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ai-asset-discovery/internal/model"
)

func (ps *ProcessScanner) listProcesses() ([]model.ProcessInfo, error) {
	// Preferred: Get-CimInstance for rich output (PID, PPID, Name, CommandLine,
	// ExecutablePath, plus owner via GetOwner()).
	if procs, err := listProcessesPowerShell(); err == nil && len(procs) > 0 {
		return procs, nil
	}
	// Fallback: tasklist.exe (PID, Name only).
	return listProcessesTasklist()
}

// listProcessesPowerShell uses Get-CimInstance Win32_Process for full metadata
// and GetOwner() for the process user.
func listProcessesPowerShell() ([]model.ProcessInfo, error) {
	// Use |-separated output to avoid comma/semicolon ambiguity in cmdlines.
	// Fields: ProcessId|ParentProcessId|Name|CommandLine|ExecutablePath|Owner
	script := `Get-CimInstance Win32_Process | ForEach-Object {` +
		`$owner = (Invoke-CimMethod -InputObject $_ -MethodName GetOwner).User;` +
		`"{0}|{1}|{2}|{3}|{4}|{5}" -f $_.ProcessId,$_.ParentProcessId,$_.Name,$_.CommandLine,$_.ExecutablePath,$owner` +
		`}`

	out, err := exec.Command(
		"powershell", "-NoProfile", "-Command", script,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("powershell: %w", err)
	}

	return parsePowerShellLines(string(out)), nil
}

// listProcessesTasklist is a fallback using tasklist.exe (bundled with Windows).
// Output format: CSV with PID, Name (no PPID, no full command line).
func listProcessesTasklist() ([]model.ProcessInfo, error) {
	out, err := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil, fmt.Errorf("tasklist: %w", err)
	}
	return parseTasklistCSV(string(out)), nil
}

func parseTasklistCSV(output string) []model.ProcessInfo {
	var procs []model.ProcessInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "ImageName","PID","SessionName","Session#","Mem Usage"
		fields := strings.Split(line, `","`)
		if len(fields) < 2 {
			continue
		}
		name := strings.Trim(fields[0], `"`)
		// Strip .exe suffix for consistency
		name = strings.TrimSuffix(name, ".exe")
		pidStr := strings.Trim(fields[1], `"`)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		procs = append(procs, model.ProcessInfo{
			PID:  pid,
			Name: name,
		})
	}
	return procs
}

func parsePowerShellLines(output string) []model.ProcessInfo {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var procs []model.ProcessInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		proc := parsePSLine6(line)
		if proc != nil {
			procs = append(procs, *proc)
		}
	}
	return procs
}

// parsePSLine6 parses the 6-field |-separated line:
// PID|PPID|Name|CommandLine|ExecutablePath|Owner
func parsePSLine6(line string) *model.ProcessInfo {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) < 3 {
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil
	}
	ppid, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

	name := strings.TrimSpace(parts[2])
	// Win32_Process.Name includes .exe suffix; strip for consistency.
	name = strings.TrimSuffix(name, ".exe")

	cmdline := name
	if len(parts) >= 4 {
		cl := strings.TrimSpace(parts[3])
		if cl != "" {
			cmdline = cl
		}
	}

	exe := ""
	if len(parts) >= 5 {
		v := strings.TrimSpace(parts[4])
		if v != "" {
			exe = v
		}
	}

	user := ""
	if len(parts) >= 6 {
		v := strings.TrimSpace(parts[5])
		if v != "" {
			user = v
		}
	}

	// Derive cwd from exe path when available — remove the trailing executable name.
	cwd := ""
	if exe != "" {
		sep := strings.LastIndexAny(exe, "\\/")
		if sep >= 0 {
			cwd = exe[:sep]
		}
	}

	return &model.ProcessInfo{
		PID:        pid,
		PPID:       ppid,
		Name:       name,
		CmdLine:    cmdline,
		User:       user,
		CWD:        cwd,
		Executable: exe,
	}
}
