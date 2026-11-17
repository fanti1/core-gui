// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

// genicon generates the icon assets and the Windows resource, with no external tools:
//
//	assets/icon.png            256px icon (README)
//	rsrc_windows_amd64.syso    Windows resource with the app icon and manifest
//	                           (picked up automatically by "go build")
//
// Usage: go generate (from the repository root).
package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"log"
	"os"

	"coregui/internal/icon"
)

//go:embed app.manifest
var manifest []byte

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtManifest  = 24
)

func main() {
	var imgs []*image.RGBA
	for _, s := range []int{16, 20, 24, 32, 40, 48, 64, 128, 256} {
		imgs = append(imgs, icon.Draw(s, icon.Green))
	}
	must(os.MkdirAll("assets", 0o755))
	must(os.WriteFile("assets/icon.png", icon.PNG(imgs[len(imgs)-1]), 0o644))

	// Resources must be sorted by type, then ID.
	var rs []resource
	var data [][]byte
	for i, img := range imgs {
		d := icon.ImageData(img)
		data = append(data, d)
		rs = append(rs, resource{rtIcon, uint32(i + 1), d})
	}
	rs = append(rs, resource{rtGroupIcon, 1, icon.GroupIcon(imgs, data, 1)})
	rs = append(rs, resource{rtManifest, 1, manifest})
	must(os.WriteFile("rsrc_windows_amd64.syso", coff(resourceSection(rs)), 0o644))
}

type resource struct {
	typ, id uint32
	data    []byte
}

type section struct {
	data   []byte
	relocs []uint32 // offsets of OffsetToData (RVA) fields the linker must fix up
}

// resourceSection builds the .rsrc tree (type -> ID -> language -> data) for resources
// sorted by type and ID.
func resourceSection(rs []resource) section {
	const (
		lang  = 0x0409
		isDir = 0x80000000
	)
	var types []uint32
	count := map[uint32]int{}
	for _, r := range rs {
		if count[r.typ] == 0 {
			types = append(types, r.typ)
		}
		count[r.typ]++
	}

	// Layout: root | one dir per type | one language dir per resource | data entries | blobs
	off := 16 + 8*len(types)
	typeDir := map[uint32]int{}
	for _, t := range types {
		typeDir[t] = off
		off += 16 + 8*count[t]
	}
	langDir := off
	entries := langDir + 24*len(rs)
	blobs := entries + 16*len(rs)

	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	// IMAGE_RESOURCE_DIRECTORY (16 bytes): characteristics, timestamp, major, minor, #named, #ids
	dir := func(ids int) { w([2]uint32{}); w([4]uint16{0, 0, 0, uint16(ids)}) }
	entry := func(id, off uint32) { w(id); w(off) }

	dir(len(types))
	for _, t := range types {
		entry(t, isDir|uint32(typeDir[t]))
	}
	i := 0
	for _, t := range types {
		dir(count[t])
		for range count[t] {
			entry(rs[i].id, isDir|uint32(langDir+24*i))
			i++
		}
	}
	for i := range rs {
		dir(1)
		entry(lang, uint32(entries+16*i))
	}
	var s section
	pos := blobs
	for i, r := range rs { // IMAGE_RESOURCE_DATA_ENTRY
		s.relocs = append(s.relocs, uint32(entries+16*i))
		w(uint32(pos)) // section offset; the linker turns it into an RVA
		w(uint32(len(r.data)))
		w([2]uint32{})
		pos += align4(len(r.data))
	}
	for _, r := range rs {
		b.Write(r.data)
		b.Write(make([]byte, align4(len(r.data))-len(r.data)))
	}
	s.data = b.Bytes()
	return s
}

// coff wraps the .rsrc section in an amd64 COFF object the Go linker accepts as a .syso.
func coff(s section) []byte {
	const (
		machineAMD64  = 0x8664
		relAddr32NB   = 3 // IMAGE_REL_AMD64_ADDR32NB
		fileHeader    = 20
		sectionHeader = 40
	)
	offData := fileHeader + sectionHeader
	offRelocs := offData + len(s.data)
	offSyms := offRelocs + 10*len(s.relocs)

	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	// file header
	w(uint16(machineAMD64))
	w(uint16(1)) // sections
	w(uint32(0)) // timestamp
	w(uint32(offSyms))
	w(uint32(1)) // symbols
	w(uint16(0)) // optional header size
	w(uint16(0)) // characteristics
	// section header
	w([8]byte{'.', 'r', 's', 'r', 'c'})
	w(uint32(0)) // virtual size
	w(uint32(0)) // virtual address
	w(uint32(len(s.data)))
	w(uint32(offData))
	w(uint32(offRelocs))
	w(uint32(0)) // line numbers
	w(uint16(len(s.relocs)))
	w(uint16(0))
	w(uint32(0x40000040)) // INITIALIZED_DATA | MEM_READ
	b.Write(s.data)
	for _, r := range s.relocs {
		w(r)         // offset in section
		w(uint32(0)) // symbol 0 (.rsrc)
		w(uint16(relAddr32NB))
	}
	// section symbol
	w([8]byte{'.', 'r', 's', 'r', 'c'})
	w(uint32(0)) // value
	w(int16(1))  // section number
	w(uint16(0)) // type
	w(uint8(3))  // IMAGE_SYM_CLASS_STATIC
	w(uint8(0))  // aux symbols
	w(uint32(4)) // empty string table
	return b.Bytes()
}

func align4(n int) int { return (n + 3) &^ 3 }

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
