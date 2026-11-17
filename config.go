// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Settings live in <user config dir>/coregui/config as key=value lines:
//
//	exclude=0|01        cores removed from CS2
//	close_to_tray=1|0   window close button hides to tray
//	lang=auto|en|pt...  interface language

var (
	exclude     atomic.Uint64 // CPUs CS2 must NOT use (excludeCore0 / excludeCore01)
	closeToTray atomic.Bool
)

func configFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "coregui", "config")
}

func loadSettings() {
	exclude.Store(excludeCore0)
	closeToTray.Store(true)
	lang := "auto"

	b, _ := os.ReadFile(configFile())
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "exclude":
			if v == "01" {
				exclude.Store(excludeCore01)
			}
		case "close_to_tray":
			closeToTray.Store(v != "0")
		case "lang":
			lang = v
		}
	}
	applyLanguage(lang)
}

func saveSettings() {
	f := configFile()
	if f == "" {
		return
	}
	ex := "0"
	if exclude.Load() == excludeCore01 {
		ex = "01"
	}
	ct := "1"
	if !closeToTray.Load() {
		ct = "0"
	}
	_ = os.MkdirAll(filepath.Dir(f), 0o755)
	_ = os.WriteFile(f, fmt.Appendf(nil, "exclude=%s\nclose_to_tray=%s\nlang=%s\n", ex, ct, languageSetting()), 0o644)
}

// Setters used by the tray and the window. Each one saves and refreshes every UI.

func setExclude(m uint64) {
	exclude.Store(m)
	saveSettings()
	kickCheck()
	refreshUI()
}

func setCloseToTray(on bool) {
	closeToTray.Store(on)
	saveSettings()
	refreshUI()
}

func setLanguage(code string) {
	applyLanguage(code)
	saveSettings()
	refreshUI()
}

func toggleAutostart(on bool) {
	_ = setAutostart(on)
	refreshUI() // shows the real state even if it failed
}
