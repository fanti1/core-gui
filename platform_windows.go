// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Minimal handle rights. No PROCESS_VM_READ / PROCESS_VM_WRITE / PROCESS_VM_OPERATION:
// with this handle Windows itself refuses any access to CS2's memory.
const affinityRights = windows.PROCESS_SET_INFORMATION | windows.PROCESS_QUERY_LIMITED_INFORMATION

var (
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procSetProcessAffinityMask = kernel32.NewProc("SetProcessAffinityMask")
	procGetProcessAffinityMask = kernel32.NewProc("GetProcessAffinityMask")
)

// findCS2 walks the process list (names and PIDs only) and returns the PIDs of cs2.exe.
func findCS2() []int {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var pids []int
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "cs2.exe") {
			pids = append(pids, int(e.ProcessID))
		}
	}
	return pids
}

// isCS2 cheaply checks that pid is still a running cs2.exe (no process list scan).
func isCS2(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != 259 /*STILL_ACTIVE*/ {
		return false
	}
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return false
	}
	path := windows.UTF16ToString(buf[:n])
	return strings.EqualFold(path[strings.LastIndexAny(path, `\/`)+1:], "cs2.exe")
}

func setAffinity(pid int, mask uint64) error {
	h, err := windows.OpenProcess(affinityRights, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if r, _, e := procSetProcessAffinityMask.Call(uintptr(h), uintptr(mask)); r == 0 {
		return e
	}
	return nil
}

func getAffinity(pid int) (uint64, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	proc, _, err := affinityMasks(h)
	return proc, err
}

// systemMask returns every logical CPU Windows makes available to processes.
func systemMask() (uint64, error) {
	_, sys, err := affinityMasks(windows.CurrentProcess())
	return sys, err
}

func affinityMasks(h windows.Handle) (proc, sys uint64, err error) {
	var p, s uintptr
	if r, _, e := procGetProcessAffinityMask.Call(uintptr(h),
		uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&s))); r == 0 {
		return 0, 0, e
	}
	return uint64(p), uint64(s), nil
}

// acquireSingleInstance holds a named mutex for the lifetime of the app.
func acquireSingleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Local\CoreGUI`)
	_, err := windows.CreateMutex(nil, false, name)
	return err != windows.ERROR_ALREADY_EXISTS
}

// Autostart: HKCU\...\Run value (per user, no admin needed).
const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "CoreGUI"
)

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValue)
	return err == nil
}

func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		return k.DeleteValue(runValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValue, `"`+exe+`" `+minimizedArg)
}

// systemLanguages returns the user's preferred UI languages, e.g. ["pt-BR", "en-US"].
func systemLanguages() []string {
	langs, _ := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	return langs
}
