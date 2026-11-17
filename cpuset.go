// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Affinity masks are uint64 bitsets: bit N set = logical CPU N allowed.
// Systems with more than 64 logical CPUs are limited to the first 64.

const (
	excludeCore0  uint64 = 0b01 // remove CPU 0
	excludeCore01 uint64 = 0b11 // remove CPUs 0 and 1
)

var errNotEnoughCPUs = errors.New("not enough CPUs")

// targetMask returns every system CPU except the excluded ones.
func targetMask(all, excluded uint64) (uint64, error) {
	want := all &^ excluded
	if want == 0 {
		return 0, errNotEnoughCPUs
	}
	return want, nil
}

// formatCPUs renders a mask as ranges: 0b1111_1110 -> "1-7".
func formatCPUs(m uint64) string {
	var parts []string
	for i := 0; i < 64; {
		if m&(1<<i) == 0 {
			i++
			continue
		}
		j := i
		for j+1 < 64 && m&(1<<(j+1)) != 0 {
			j++
		}
		if i == j {
			parts = append(parts, strconv.Itoa(i))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", i, j))
		}
		i = j + 1
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// parseCPUs parses the Linux CPU list format ("0-3,8,10-11") into a mask.
func parseCPUs(s string) (uint64, error) {
	var m uint64
	for _, part := range strings.Split(strings.TrimSpace(s), ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return 0, err
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil {
				return 0, err
			}
		}
		for i := a; i <= b && i < 64; i++ {
			m |= 1 << i
		}
	}
	return m, nil
}
