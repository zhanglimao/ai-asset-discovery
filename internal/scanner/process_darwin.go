// Package scanner — macOS process listing via ps(1) + lsof(8) enrichment.
package scanner

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/ai-asset-discovery/internal/model"
)

func (ps *ProcessScanner) listProcesses() ([]model.ProcessInfo, error) {
	// ps -eo pid,ppid,user,comm,args — BSD-style column format,
	// `comm` = basename, `args` = full command line.
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

	// Enrich with cwd / exe via lsof in parallel (expensive but accurate).
	ps.enrichMacOSProcesses(procs)

	return procs, nil
}

// enrichMacOSProcesses fills CWD and Executable for macOS processes
// using per-pid lsof queries executed concurrently.
func (ps *ProcessScanner) enrichMacOSProcesses(procs []model.ProcessInfo) {
	if len(procs) == 0 {
		return
	}

	type result struct {
		idx int
		cwd string
		exe string
	}

	var wg sync.WaitGroup
	// Cap concurrency to avoid overwhelming the system.
	sem := make(chan struct{}, 32)
	ch := make(chan result, len(procs))

	for i := range procs {
		if procs[i].PID <= 1 {
			continue // skip kernel / launchd
		}
		wg.Add(1)
		go func(idx, pid int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cwd, exe := lsofProcInfo(pid)
			ch <- result{idx: idx, cwd: cwd, exe: exe}
		}(i, procs[i].PID)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		if r.cwd != "" {
			procs[r.idx].CWD = r.cwd
		}
		if r.exe != "" {
			procs[r.idx].Executable = r.exe
		}
	}
}

// lsofProcInfo returns (cwd, exe) for a single pid via lsof.
// lsof -Fn output uses field-identifier prefixes:
//
//	p<PID>
//	f<cwd|txt|...>
//	n<path>
func lsofProcInfo(pid int) (cwd, exe string) {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd,txt", "-Fn").Output()
	if err != nil {
		return "", ""
	}
	var curType string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "fcwd"):
			curType = "cwd"
		case strings.HasPrefix(line, "ftxt"):
			curType = "txt"
		case strings.HasPrefix(line, "n") && curType != "":
			val := line[1:]
			switch curType {
			case "cwd":
				cwd = val
			case "txt":
				exe = val
			}
			curType = ""
		default:
			curType = ""
		}
	}
	return cwd, exe
}

// parsePSLine parses one line of `ps -eo pid,ppid,user,comm,args`.
// Fields are space-separated; args can contain spaces.
// Strategy: use column offsets based on ps's fixed-width output style.
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
	name := fields[3]
	cmdline := name
	if len(fields) > 4 {
		cmdline = strings.Join(fields[4:], " ")
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
