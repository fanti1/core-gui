// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

// Package canvas draws anti-aliased shapes into a 32-bit BGRA pixel buffer
// (the layout of a Windows top-down DIB section). Text is drawn by the OS on top.
package canvas

import "math"

type Color struct{ R, G, B uint8 }

// Hex builds a Color from 0xRRGGBB.
func Hex(v uint32) Color { return Color{uint8(v >> 16), uint8(v >> 8), uint8(v)} }

// Canvas wraps a BGRA buffer of W×H pixels, rows Stride bytes apart.
type Canvas struct {
	Pix          []byte
	W, H, Stride int
}

func (c *Canvas) Fill(col Color) {
	for y := 0; y < c.H; y++ {
		row := c.Pix[y*c.Stride:]
		for x := 0; x < c.W; x++ {
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = col.B, col.G, col.R, 255
		}
	}
}

// blend mixes col into pixel (x,y) with coverage a (0..1).
func (c *Canvas) blend(x, y int, col Color, a float64) {
	if a <= 0 || x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	i := y*c.Stride + x*4
	if a >= 1 {
		c.Pix[i], c.Pix[i+1], c.Pix[i+2] = col.B, col.G, col.R
		return
	}
	mix := func(d, s uint8) uint8 { return uint8(float64(d)*(1-a) + float64(s)*a + 0.5) }
	c.Pix[i], c.Pix[i+1], c.Pix[i+2] = mix(c.Pix[i], col.B), mix(c.Pix[i+1], col.G), mix(c.Pix[i+2], col.R)
}

// RoundRect fills the rectangle [x0,x1)×[y0,y1) with corner radius r, anti-aliased.
func (c *Canvas) RoundRect(x0, y0, x1, y1, r float64, col Color) {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hw, hh := (x1-x0)/2, (y1-y0)/2
	r = math.Min(r, math.Min(hw, hh))
	for y := int(math.Floor(y0)); y < int(math.Ceil(y1)); y++ {
		for x := int(math.Floor(x0)); x < int(math.Ceil(x1)); x++ {
			// signed distance from the pixel center to the rounded rectangle
			qx := math.Abs(float64(x)+0.5-cx) - hw + r
			qy := math.Abs(float64(y)+0.5-cy) - hh + r
			d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
			c.blend(x, y, col, math.Min(1, 0.5-d))
		}
	}
}

// Circle fills a circle centered at (cx,cy) with radius r, anti-aliased.
func (c *Canvas) Circle(cx, cy, r float64, col Color) {
	c.RoundRect(cx-r, cy-r, cx+r, cy+r, r, col)
}

// Border draws a rounded rectangle outline of width w by filling the outer shape
// with border and the inner one with fill.
func (c *Canvas) Border(x0, y0, x1, y1, r, w float64, border, fill Color) {
	c.RoundRect(x0, y0, x1, y1, r, border)
	c.RoundRect(x0+w, y0+w, x1-w, y1-w, math.Max(0, r-w), fill)
}
