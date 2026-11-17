// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"image/color"
	"runtime"

	"coregui/internal/icon"
)

// Tray icons: the same chip, colored by state (gray idle, green OK, red error).
type iconSet struct{ idle, ok, err []byte }

func makeIcons() iconSet {
	return iconSet{idle: trayIcon(icon.Gray), ok: trayIcon(icon.Green), err: trayIcon(icon.Red)}
}

func trayIcon(c color.RGBA) []byte {
	if runtime.GOOS == "windows" {
		return icon.ICO(icon.Draw(16, c), icon.Draw(24, c), icon.Draw(32, c), icon.Draw(48, c))
	}
	return icon.PNG(icon.Draw(64, c))
}
