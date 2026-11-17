// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

// Native Windows tray icon (Shell_NotifyIconW), owned by the settings window and
// driven by its message loop. The context menu is rebuilt on every right click, so
// it always shows the current state and language.

import (
	"image/color"
	"unsafe"

	"coregui/internal/icon"
	"golang.org/x/sys/windows"
)

var (
	pShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	pCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	pAppendMenuW              = user32.NewProc("AppendMenuW")
	pTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	pDestroyMenu              = user32.NewProc("DestroyMenu")
	pGetCursorPos             = user32.NewProc("GetCursorPos")
	pRegisterWindowMessageW   = user32.NewProc("RegisterWindowMessageW")
	pCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	pSetMenuDefaultItem       = user32.NewProc("SetMenuDefaultItem")
)

const (
	wmTray        = 0x8000 + 3 // WM_APP + 3: tray icon callback
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmContextMenu = 0x007B

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2
	nifMsg    = 0x1
	nifIcon   = 0x2
	nifTip    = 0x4

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfSeparator = 0x0800
	tpmRetCmd   = 0x0100
	tpmRightBtn = 0x0002
)

// Tray menu command IDs.
const (
	cmdOpen = 1 + iota
	cmdCore0
	cmdCore01
	cmdAutostart
	cmdCloseToTray
	cmdExit
)

type notifyIconData struct {
	size                uint32
	hwnd                uintptr
	id, flags, callback uint32
	icon                uintptr
	tip                 [128]uint16
	state, stateMask    uint32
	info                [256]uint16
	version             uint32
	infoTitle           [64]uint16
	infoFlags           uint32
	guid                windows.GUID
	balloonIcon         uintptr
}

type trayIcon struct {
	icons          map[state]uintptr
	shown          state
	taskbarCreated uintptr // message Explorer broadcasts after restarting
}

var tray trayIcon

// init creates one HICON per state at the exact small-icon size (crisp at any DPI).
func (t *trayIcon) init() {
	size := int(call(pGetSystemMetrics, 49)) // SM_CXSMICON
	t.icons = map[state]uintptr{}
	for st, c := range map[state]color.RGBA{stateIdle: icon.Gray, stateOK: icon.Green, stateError: icon.Red} {
		bits := icon.ImageData(icon.Draw(size, c)) // DIB = icon resource format
		t.icons[st] = call(pCreateIconFromResourceEx, uintptr(unsafe.Pointer(&bits[0])), uintptr(len(bits)),
			1 /*fIcon*/, 0x00030000 /*version*/, uintptr(size), uintptr(size), 0)
	}
	t.taskbarCreated = call(pRegisterWindowMessageW, utf16("TaskbarCreated"))
}

func (t *trayIcon) data(flags uint32) *notifyIconData {
	s := currentStatus()
	d := &notifyIconData{hwnd: win.hwnd, id: 1, flags: flags, callback: wmTray, icon: t.icons[s.state]}
	d.size = uint32(unsafe.Sizeof(*d))
	tip, _ := windows.UTF16FromString(tooltip(s))
	if len(tip) > len(d.tip) {
		tip = append(tip[:len(d.tip)-1], 0)
	}
	copy(d.tip[:], tip)
	t.shown = s.state
	return d
}

func (t *trayIcon) add() {
	call(pShellNotifyIconW, nimAdd, uintptr(unsafe.Pointer(t.data(nifMsg|nifIcon|nifTip))))
}

func (t *trayIcon) update() {
	call(pShellNotifyIconW, nimModify, uintptr(unsafe.Pointer(t.data(nifIcon|nifTip))))
}

func (t *trayIcon) remove() {
	call(pShellNotifyIconW, nimDelete, uintptr(unsafe.Pointer(t.data(0))))
}

// menu shows the right-click menu at the cursor and runs the chosen command.
func (t *trayIcon) menu() {
	m := call(pCreatePopupMenu)
	add := func(flags uintptr, id uintptr, text string) { call(pAppendMenuW, m, flags, id, utf16(text)) }
	check := func(on bool) uintptr {
		if on {
			return mfChecked
		}
		return 0
	}
	s := currentStatus()
	main, detail := statusLines(s)
	if detail == "" {
		detail = plannedSummary()
	}
	add(mfString, cmdOpen, T("open"))
	call(pSetMenuDefaultItem, m, cmdOpen, 0) // bold, like most tray apps
	add(mfSeparator, 0, "")
	add(mfString|mfGrayed, 0, main)
	add(mfString|mfGrayed, 0, detail)
	add(mfSeparator, 0, "")
	add(mfString|check(exclude.Load() == excludeCore0), cmdCore0, T("remove0"))
	add(mfString|check(exclude.Load() == excludeCore01), cmdCore01, T("remove01"))
	add(mfSeparator, 0, "")
	add(mfString|check(autostartEnabled()), cmdAutostart, T("autostart"))
	add(mfString|check(closeToTray.Load()), cmdCloseToTray, T("close_tray"))
	add(mfSeparator, 0, "")
	add(mfString, cmdExit, T("exit"))

	var pt [2]int32
	call(pGetCursorPos, uintptr(unsafe.Pointer(&pt)))
	call(pSetForegroundWindow, win.hwnd) // required so the menu closes when clicking elsewhere
	cmd := call(pTrackPopupMenu, m, tpmRetCmd|tpmRightBtn, uintptr(pt[0]), uintptr(pt[1]), 0, win.hwnd, 0)
	call(pDestroyMenu, m)
	call(pPostMessageW, win.hwnd, 0, 0, 0) // WM_NULL, per TrackPopupMenu docs

	switch cmd {
	case cmdOpen:
		win.show()
	case cmdCore0:
		go setExclude(excludeCore0)
	case cmdCore01:
		go setExclude(excludeCore01)
	case cmdAutostart:
		go toggleAutostart(!autostartEnabled())
	case cmdCloseToTray:
		go setCloseToTray(!closeToTray.Load())
	case cmdExit:
		quit()
	}
}
