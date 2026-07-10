// Package scanner — Windows process listing via PowerShell (Get-CimInstance).
// Uses PowerShell instead of tasklist.exe for richer output (PID, PPID, user, command line).
package scanner

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ai-asset-discovery/internal/model"
)

func (ps *ProcessScanner) listProcesses() ([]model.ProcessInfo, error) {
	// Get-CimInstance produces CSV with predictable columns.
	// Using CIM instead of Get-Process for PPID support.
	out, err := exec.Command(
		"powershell", "-NoProfile", "-Command",
		// We format as |-separated to avoid comma/semicolon ambiguity in cmdlines.
		`Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId,Name,CommandLine | ForEach-Object { "{0}|{1}|{2}|{3}" -f $_.ProcessId,$_.ParentProcessId,$_.Name,$_.CommandLine }`,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("powershell: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var procs []model.ProcessInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		proc := parsePowerShellLine(line)
		if proc != nil {
			procs = append(procs, *proc)
		}
	}
	return procs, nil
}

func parsePowerShellLine(line string) *model.ProcessInfo {
	// Format: PID|PPID|Name|CommandLine
	parts := strings.SplitN(line, "|", 4)
	if len(parts) < 3 {
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil
	}
	ppid, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

	name := strings.TrimSpace(parts[2])
	cmdline := name
	if len(parts) >= 4 {
		cmdline = strings.TrimSpace(parts[3])
	}

	return &model.ProcessInfo{
		PID:        pid,
		PPID:       ppid,
		Name:       name,
		CmdLine:    cmdline,
		User:       "", // Win32_Process doesn't carry user; use Get-Process -IncludeUserName if needed later
		CWD:        "",
		Executable: "",
	}
}
