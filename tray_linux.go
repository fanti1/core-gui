// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

// Linux UI: tray icon and menu only (StatusNotifierItem via fyne.io/systray).
// There is no settings window, which keeps the app free of GTK/Qt; the menu has
// every option, including the language submenu.

import (
	"sync"
	"sync/atomic"

	"fyne.io/systray"
)

var trayUI atomic.Pointer[tray]

type tray struct {
	mu                 sync.Mutex
	icons              iconSet
	lastIcon           state
	status, detail     *systray.MenuItem
	rm0, rm01          *systray.MenuItem
	autostart, exit    *systray.MenuItem
	langMenu, langAuto *systray.MenuItem
	langItems          []*systray.MenuItem // one per language
}

// runUI runs the tray until the user exits. Linux has no window, so show is ignored.
func runUI(show bool) { systray.Run(onReady, nil) }

// refreshUI redraws the tray from the current state. Safe from any goroutine.
func refreshUI() {
	if t := trayUI.Load(); t != nil {
		t.refresh()
	}
}

func showExistingInstance() {}

func onReady() {
	t := &tray{icons: makeIcons(), lastIcon: -1}

	t.status = systray.AddMenuItem("", "")
	t.status.Disable()
	t.detail = systray.AddMenuItem("", "")
	t.detail.Disable()
	systray.AddSeparator()
	t.rm0 = systray.AddMenuItemCheckbox("", "", false)
	t.rm01 = systray.AddMenuItemCheckbox("", "", false)
	systray.AddSeparator()
	t.autostart = systray.AddMenuItemCheckbox("", "", false)
	t.langMenu = systray.AddMenuItem("", "")
	t.langAuto = t.langMenu.AddSubMenuItemCheckbox("", "", false)
	for _, l := range languages {
		t.langItems = append(t.langItems, t.langMenu.AddSubMenuItemCheckbox(l.name, "", false))
	}
	systray.AddSeparator()
	t.exit = systray.AddMenuItem("", "")

	trayUI.Store(t)
	t.refresh()

	onClick(t.rm0, func() { setExclude(excludeCore0) })
	onClick(t.rm01, func() { setExclude(excludeCore01) })
	onClick(t.autostart, func() { toggleAutostart(!autostartEnabled()) })
	onClick(t.langAuto, func() { setLanguage("auto") })
	for i, item := range t.langItems {
		code := languages[i].code
		onClick(item, func() { setLanguage(code) })
	}
	onClick(t.exit, systray.Quit)

	go run()
}
func onClick(item *systray.MenuItem, f func()) {
	if item == nil {
		return
	}
	go func() {
		for range item.ClickedCh {
			f()
		}
	}()
}

// refresh updates every tray text, check mark and the icon from the current state.
func (t *tray) refresh() {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := currentStatus()
	if s.state != t.lastIcon {
		systray.SetIcon(map[state][]byte{stateIdle: t.icons.idle, stateOK: t.icons.ok, stateError: t.icons.err}[s.state])
		t.lastIcon = s.state
	}
	systray.SetTooltip(tooltip(s))

	main, detail := statusLines(s)
	if detail == "" {
		detail = plannedSummary()
	}
	t.status.SetTitle(main)
	t.detail.SetTitle(detail)
	t.rm0.SetTitle(T("remove0"))
	setChecked(t.rm0, exclude.Load() == excludeCore0)
	t.rm01.SetTitle(T("remove01"))
	setChecked(t.rm01, exclude.Load() == excludeCore01)
	t.autostart.SetTitle(T("autostart"))
	setChecked(t.autostart, autostartEnabled())
	t.langMenu.SetTitle(T("language"))
	t.langAuto.SetTitle(T("auto"))
	setChecked(t.langAuto, languageSetting() == "auto")
	for i, item := range t.langItems {
		setChecked(item, languageSetting() == languages[i].code)
	}
	t.exit.SetTitle(T("exit"))
}

func setChecked(item *systray.MenuItem, on bool) {
	if on {
		item.Check()
	} else {
		item.Uncheck()
	}
}
