// Package scanner — macOS process listing via ps(1).
package scanner

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ai-asset-discovery/internal/model"
)

func (ps *ProcessScanner) listProcesses() ([]model.ProcessInfo, error) {
	// ps -eo pid,ppid,user,comm,args — BSD-style column format,
	// `comm` = basename, `args` = full command line.
	// We need PPID first so pid is after it for consistent parsing.
	out, err := exec.Command("ps", "-eo", "pid,ppid,user,comm,args").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil, nil // header only
	}

	var procs []model.ProcessInfo
	for _, line := range lines[1:] {
		proc := parsePSLine(line)
		if proc != nil {
			procs = append(procs, *proc)
		}
	}
	return procs, nil
}

// parsePSLine parses one line of `ps -eo pid,ppid,user,comm,args`.
// Fields are space-separated; args can contain spaces so we split
// cautiously from the left.
func parsePSLine(line string) *model.ProcessInfo {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return nil
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		ppid = 0
	}
	user := fields[2]
	// On macOS ps, `comm` is truncated to the first 20 chars of the command name
	// — we may not get `comm` as a separate field if args contains it.
	// fields[3] is comm, fields[4:] is args.
	// But ps may not include comm separately; we merge.
	name := fields[3]
	cmdline := name
	if len(fields) > 4 {
		cmdline = strings.Join(fields[4:], " ")
	} else if len(fields) == 4 {
		// comm == args? Just use it.
		cmdline = name
	}

	return &model.ProcessInfo{
		PID:        pid,
		PPID:       ppid,
		Name:       name,
		CmdLine:    cmdline,
		User:       user,
		CWD:        "",
		Executable: "",
	}
}
