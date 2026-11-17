// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// findCS2 reads only /proc/<pid>/comm (the process name). Never /proc/<pid>/mem.
func findCS2() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err == nil && strings.TrimSpace(string(b)) == "cs2" {
			pids = append(pids, pid)
		}
	}
	return pids
}

// isCS2 cheaply checks that pid is still a running cs2 process (no /proc scan).
func isCS2(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	return err == nil && strings.TrimSpace(string(b)) == "cs2"
}

// On Linux affinity is per thread, so it is applied to every thread of the process.
// New threads inherit it from their creator; the loop re-applies if any escapes.
func threads(pid int) []int {
	entries, err := os.ReadDir("/proc/" + strconv.Itoa(pid) + "/task")
	if err != nil {
		return []int{pid}
	}
	tids := make([]int, 0, len(entries))
	for _, e := range entries {
		if tid, err := strconv.Atoi(e.Name()); err == nil {
			tids = append(tids, tid)
		}
	}
	return tids
}

func setAffinity(pid int, mask uint64) error {
	var set unix.CPUSet
	for i := 0; i < 64; i++ {
		if mask&(1<<i) != 0 {
			set.Set(i)
		}
	}
	var mainErr error
	for _, tid := range threads(pid) {
		// Threads may exit mid-loop; only a failure on the main thread is reported.
		if err := unix.SchedSetaffinity(tid, &set); err != nil && tid == pid {
			mainErr = err
		}
	}
	return mainErr
}

// getAffinity returns the UNION of all thread masks: if any thread is outside
// the target, the result differs and the loop re-applies.
func getAffinity(pid int) (uint64, error) {
	var union uint64
	for _, tid := range threads(pid) {
		var set unix.CPUSet
		if err := unix.SchedGetaffinity(tid, &set); err != nil {
			if tid == pid {
				return 0, err
			}
			continue
		}
		union |= toMask(&set)
	}
	return union, nil
}

// systemMask returns the online CPUs, falling back to this app's own affinity.
func systemMask() (uint64, error) {
	if b, err := os.ReadFile("/sys/devices/system/cpu/online"); err == nil {
		if m, err := parseCPUs(string(b)); err == nil {
			return m, nil
		}
	}
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return 0, err
	}
	return toMask(&set), nil
}

func toMask(set *unix.CPUSet) uint64 {
	var m uint64
	for i := 0; i < 64; i++ {
		if set.IsSet(i) {
			m |= 1 << i
		}
	}
	return m
}

// acquireSingleInstance binds an abstract unix socket; the kernel releases it on exit.
var instanceLock net.Listener

func acquireSingleInstance() bool {
	l, err := net.Listen("unix", "@coregui")
	if err != nil {
		return false
	}
	instanceLock = l
	return true
}

// Autostart: XDG autostart entry (~/.config/autostart).
func desktopFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "autostart", "coregui.desktop")
}

func autostartEnabled() bool {
	_, err := os.Stat(desktopFile())
	return err == nil
}

func setAutostart(on bool) error {
	f := desktopFile()
	if !on {
		return os.Remove(f)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	content := "[Desktop Entry]\nType=Application\nName=CoreGUI\nExec=\"" + exe + "\" " + minimizedArg + "\nX-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(f, []byte(content), 0o644)
}

// systemLanguages returns the locale from the environment, e.g. ["pt_BR.UTF-8"].
func systemLanguages() []string {
	var langs []string
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			langs = append(langs, v)
		}
	}
	return langs
}
