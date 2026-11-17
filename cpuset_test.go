// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import "testing"

func TestTargetMask(t *testing.T) {
	cases := []struct {
		all, excl, want uint64
		fails           bool
	}{
		{0xFF, excludeCore0, 0xFE, false},     // 8 threads  -> 1-7
		{0xFF, excludeCore01, 0xFC, false},    // 8 threads  -> 2-7
		{0xFFFF, excludeCore0, 0xFFFE, false}, // 16 threads -> 1-15
		{0b11, excludeCore01, 0, true},        // 2 threads, nothing left
		{0b1, excludeCore0, 0, true},          // 1 thread, nothing left
	}
	for _, c := range cases {
		got, err := targetMask(c.all, c.excl)
		if (err != nil) != c.fails || got != c.want {
			t.Errorf("targetMask(%#x, %#x) = %#x, %v; want %#x, fails=%v", c.all, c.excl, got, err, c.want, c.fails)
		}
	}
}

func TestFormatCPUs(t *testing.T) {
	cases := map[uint64]string{
		0:           "-",
		0b1:         "0",
		0xFFFE:      "1-15",
		0b1011_0110: "1-2, 4-5, 7",
		1 << 63:     "63",
	}
	for in, want := range cases {
		if got := formatCPUs(in); got != want {
			t.Errorf("formatCPUs(%#b) = %q; want %q", in, got, want)
		}
	}
}

func TestParseCPUs(t *testing.T) {
	cases := map[string]uint64{
		"0-15\n":      0xFFFF,
		"0":           0b1,
		"0-3,8,10-11": 0b1101_0000_1111,
	}
	for in, want := range cases {
		if got, err := parseCPUs(in); err != nil || got != want {
			t.Errorf("parseCPUs(%q) = %#b, %v; want %#b", in, got, err, want)
		}
	}
	if _, err := parseCPUs("abc"); err == nil {
		t.Error("parseCPUs(\"abc\") should fail")
	}
}
