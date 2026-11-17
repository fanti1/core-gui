// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import "testing"

// Every language must define every key the English table has.
func TestTranslationsComplete(t *testing.T) {
	for _, l := range languages {
		for k := range languages[0].text {
			if _, ok := l.text[k]; !ok {
				t.Errorf("language %s is missing %q", l.code, k)
			}
		}
	}
}
