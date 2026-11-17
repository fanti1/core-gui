// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

// CoreGUI removes Core 0 (or Cores 0 and 1) from Counter-Strike 2 automatically.
//
// CS2 keeps every other logical CPU the system has. The app never reads or writes
// process memory: it only finds the process by name and asks the OS to change and
// report its CPU affinity.
//
// Technical details: docs/TECHNICAL.md.
//
// Layout:
//
//	main.go            entry point
//	window_windows.go  settings window (native Win32, no GUI framework)
//	tray_windows.go    native Windows tray icon and menu
//	tray_linux.go      Linux tray icon and menu (fyne.io/systray; no window)
//	ui.go              status texts shared by tray and window
//	i18n.go            built-in translations
//	enforcer.go        detection loop: compute target mask, apply, verify
//	cpuset.go          CPU mask helpers (pure, tested)
//	config.go          persisted settings
//	platform_*.go      OS-specific: process list, affinity, single instance, autostart
//	internal/icon      icon drawing and ICO/DIB encoding
//	tools/genicon      generates assets/icon.png and the Windows .syso resource
package main

//go:generate go run ./tools/genicon

import (
	"os"
	"slices"
)

const (
	appName      = "CoreGUI"
	minimizedArg = "--minimized" // used by autostart: start in the tray, window hidden
)

// appVersion is set by the release build: -ldflags "-X main.appVersion=1.2.3".
var appVersion = "dev"

func main() {
	if !acquireSingleInstance() {
		showExistingInstance() // bring the running instance's window to front
		return
	}
	loadSettings()
	runUI(!slices.Contains(os.Args[1:], minimizedArg))
}
