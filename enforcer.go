// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const pollInterval = 3 * time.Second

var (
	kick       = make(chan struct{}, 1) // triggers an immediate check
	lastStatus atomic.Pointer[status]

	errRejected = errors.New("rejected")
)

func kickCheck() {
	select {
	case kick <- struct{}{}:
	default:
	}
}

type state int

const (
	stateIdle  state = iota // CS2 is not running
	stateOK                 // affinity applied and verified
	stateError              // could not apply or verify
)

type status struct {
	state state
	pids  []int
	mask  uint64
	err   error
}

func currentStatus() status {
	if s := lastStatus.Load(); s != nil {
		return *s
	}
	return status{state: stateIdle}
}

// run checks CS2 every pollInterval (or when kicked) and refreshes the UI when the status changes.
func run() {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	last := ""
	for {
		s := check()
		if key := fmt.Sprint(s, exclude.Load()); key != last {
			lastStatus.Store(&s)
			refreshUI()
			last = key
		}
		select {
		case <-t.C:
		case <-kick:
		}
	}
}

// knownPIDs caches the CS2 processes found by the last full scan. While they are
// still alive, each check only re-verifies them instead of listing every process.
// Only the run goroutine touches it.
var knownPIDs []int

func cs2PIDs() []int {
	alive := len(knownPIDs) > 0
	for _, pid := range knownPIDs {
		alive = alive && isCS2(pid)
	}
	if !alive {
		knownPIDs = findCS2()
	}
	return knownPIDs
}

// check finds CS2, applies the target affinity if needed and reads it back to verify.
func check() status {
	pids := cs2PIDs()
	if len(pids) == 0 {
		return status{state: stateIdle}
	}
	all, err := systemMask()
	if err != nil {
		return status{state: stateError, pids: pids, err: err}
	}
	want, err := targetMask(all, exclude.Load())
	if err != nil {
		return status{state: stateError, pids: pids, err: err}
	}

	s := status{state: stateOK, pids: pids}
	for _, pid := range pids {
		cur, err := getAffinity(pid)
		if err != nil || cur != want {
			if err = setAffinity(pid, want); err == nil {
				cur, err = getAffinity(pid)
			}
		}
		s.mask = cur
		switch {
		case err != nil:
			s.state, s.err = stateError, err
		case cur != want:
			s.state, s.err = stateError, errRejected
		}
	}
	return s
}
