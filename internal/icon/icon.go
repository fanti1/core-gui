// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

// Package icon draws the app icon (a CPU chip with a "0") in code, anti-aliased
// by supersampling, and encodes it as ICO/DIB/PNG. Used by the tray and by genicon.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

var (
	Green = color.RGBA{0x2e, 0xd5, 0x73, 0xff}
	Gray  = color.RGBA{0x8a, 0x91, 0x9e, 0xff}
	Red   = color.RGBA{0xff, 0x4d, 0x4f, 0xff}

	pinColor  = color.RGBA{0xc9, 0xcf, 0xd8, 0xff}
	bodyColor = color.RGBA{0x1b, 0x1f, 0x29, 0xff}
)

// Draw renders the icon at size×size pixels with accent as the highlight color.
func Draw(size int, accent color.RGBA) *image.RGBA {
	const ss = 4 // 4×4 samples per pixel
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					if c, ok := sample(x, y, accent); ok {
						r += float64(c.R)
						g += float64(c.G)
						b += float64(c.B)
						a += 255
					}
				}
			}
			n := float64(ss * ss)
			img.SetRGBA(px, py, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	return img
}

// sample returns the color at (x,y) in normalized 0..1 coordinates.
func sample(x, y float64, accent color.RGBA) (color.RGBA, bool) {
	// central "0" (elliptic ring)
	if ring(x, y, 0.5, 0.5, 0.12, 0.17, 0.065) {
		return accent, true
	}
	// chip body: colored border + dark inside
	if roundRect(x, y, 0.20, 0.20, 0.80, 0.80, 0.08) {
		return bodyColor, true
	}
	if roundRect(x, y, 0.16, 0.16, 0.84, 0.84, 0.12) {
		return accent, true
	}
	// pins: 3 per side
	for _, c := range []float64{0.32, 0.5, 0.68} {
		const w, in, out = 0.045, 0.2, 0.04
		if roundRect(x, y, c-w, out, c+w, in, 0.03) || roundRect(x, y, c-w, 1-in, c+w, 1-out, 0.03) ||
			roundRect(x, y, out, c-w, in, c+w, 0.03) || roundRect(x, y, 1-in, c-w, 1-out, c+w, 0.03) {
			return pinColor, true
		}
	}
	return color.RGBA{}, false
}

func roundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return math.Hypot(x-cx, y-cy) <= r
}

func ring(x, y, cx, cy, rx, ry, t float64) bool {
	e := func(a, b float64) float64 { return math.Pow((x-cx)/a, 2) + math.Pow((y-cy)/b, 2) }
	return e(rx+t/2, ry+t/2) <= 1 && e(rx-t/2, ry-t/2) >= 1
}

// PNG encodes an image as PNG.
func PNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// ImageData returns icon image bytes: 32-bit DIB (best compatibility) up to
// 128px, PNG for 256px.
func ImageData(img *image.RGBA) []byte {
	s := img.Bounds().Dx()
	if s >= 256 {
		return PNG(img)
	}
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	maskRow := ((s + 31) / 32) * 4
	w(uint32(40))   // BITMAPINFOHEADER
	w(int32(s))     // width
	w(int32(s * 2)) // height (color + mask)
	w(uint16(1))    // planes
	w(uint16(32))   // bpp
	w(uint32(0))    // BI_RGB
	w(uint32(s*s*4 + maskRow*s))
	w([4]uint32{})                // resolution/palette
	for y := s - 1; y >= 0; y-- { // bottom-up BGRA, straight alpha
		for x := 0; x < s; x++ {
			c := img.RGBAAt(x, y)
			if c.A > 0 && c.A < 255 {
				c.R = uint8(int(c.R) * 255 / int(c.A))
				c.G = uint8(int(c.G) * 255 / int(c.A))
				c.B = uint8(int(c.B) * 255 / int(c.A))
			}
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, maskRow*s)) // empty AND mask (alpha is used)
	return b.Bytes()
}

// writeEntry writes the fields shared by ICONDIRENTRY and GRPICONDIRENTRY.
func writeEntry(w func(any), img *image.RGBA, size int) {
	s := img.Bounds().Dx()
	if s >= 256 {
		s = 0 // 0 means 256
	}
	w(uint8(s))
	w(uint8(s))
	w(uint8(0))  // palette
	w(uint8(0))  // reserved
	w(uint16(1)) // planes
	w(uint16(32))
	w(uint32(size))
}

// ICO builds a multi-resolution .ico file.
func ICO(imgs ...*image.RGBA) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	w([3]uint16{0, 1, uint16(len(imgs))})
	data := make([][]byte, len(imgs))
	offset := 6 + 16*len(imgs)
	for i, img := range imgs {
		data[i] = ImageData(img)
		writeEntry(w, img, len(data[i]))
		w(uint32(offset))
		offset += len(data[i])
	}
	for _, d := range data {
		b.Write(d)
	}
	return b.Bytes()
}

// GroupIcon builds an RT_GROUP_ICON resource pointing to RT_ICON IDs firstID, firstID+1...
func GroupIcon(imgs []*image.RGBA, data [][]byte, firstID uint16) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	w([3]uint16{0, 1, uint16(len(imgs))})
	for i, img := range imgs {
		writeEntry(w, img, len(data[i]))
		w(firstID + uint16(i))
	}
	return b.Bytes()
}
