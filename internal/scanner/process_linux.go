// Package scanner — Linux process listing via /proc.
package scanner

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ai-asset-discovery/internal/model"
)

// listProcesses reads /proc on Linux.
func (ps *ProcessScanner) listProcesses() ([]model.ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	var procs []model.ProcessInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		proc := ps.readProc(pid)
		if proc != nil {
			procs = append(procs, *proc)
		}
	}
	return procs, nil
}

func (ps *ProcessScanner) readProc(pid int) *model.ProcessInfo {
	base := fmt.Sprintf("/proc/%d", pid)

	// Read comm (process name)
	comm, err := os.ReadFile(base + "/comm")
	if err != nil {
		return nil
	}
	name := strings.TrimSpace(string(comm))

	// Read cmdline
	cmdlineBytes, err := os.ReadFile(base + "/cmdline")
	if err != nil {
		return nil
	}
	// cmdline is null-separated
	cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)

	// Read cwd (symbolic link)
	cwd, err := os.Readlink(base + "/cwd")
	if err != nil {
		cwd = ""
	}

	// Read exe (symbolic link)
	exe, err := os.Readlink(base + "/exe")
	if err != nil {
		exe = ""
	}

	// Read status for PPID and UID
	statusBytes, err := os.ReadFile(base + "/status")
	var ppid int
	user := "unknown"
	if err == nil {
		for _, line := range strings.Split(string(statusBytes), "\n") {
			if strings.HasPrefix(line, "PPid:") {
				fmt.Sscanf(line, "PPid:\t%d", &ppid)
			}
			if strings.HasPrefix(line, "Uid:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					user = parts[1]
				}
			}
		}
	}

	return &model.ProcessInfo{
		PID:        pid,
		Name:       name,
		CmdLine:    cmdline,
		CWD:        cwd,
		Executable: exe,
		PPID:       ppid,
		User:       user,
	}
}
