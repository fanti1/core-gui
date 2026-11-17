// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"errors"
	"fmt"
	"strings"
)

// Shared text for the tray and the window, always in the current language.

// statusLines returns the main status line and the detail line ("" when idle).
func statusLines(s status) (main, detail string) {
	switch s.state {
	case stateOK:
		return Tf("running_ok", excludedName(), joinInts(s.pids)), Tf("verified", formatCPUs(s.mask))
	case stateError:
		return Tf("running_err", joinInts(s.pids)), Tf("error", errText(s.err))
	}
	return T("waiting"), ""
}

func tooltip(s status) string {
	switch s.state {
	case stateOK:
		return appName + " — " + Tf("tip_ok", excludedName())
	case stateError:
		return appName + " — " + Tf("tip_err", errText(s.err))
	}
	return appName + " — " + T("tip_wait")
}

// plannedSummary describes what applies on this machine, e.g. "CS2 will use: CPU 1-15 (of 0-15)".
func plannedSummary() string {
	all, err := systemMask()
	if err != nil {
		return T("cpus_unknown")
	}
	want, err := targetMask(all, exclude.Load())
	if err != nil {
		return Tf("error", errText(err))
	}
	return Tf("planned", formatCPUs(want), formatCPUs(all))
}

func excludedName() string {
	if exclude.Load() == excludeCore01 {
		return T("core01")
	}
	return T("core0")
}

func errText(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errNotEnoughCPUs):
		return T("not_enough")
	case errors.Is(err, errRejected):
		return T("rejected")
	}
	return err.Error() // OS errors are already localized by the OS
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = fmt.Sprint(n)
	}
	return strings.Join(s, ", ")
}
