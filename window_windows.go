// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

// Native Win32 window with a custom-drawn dark UI (no GUI framework, no child
// controls). Shapes are drawn anti-aliased in Go into a DIB section (internal/canvas),
// text is drawn on top with GDI (ClearType), and the result is blitted in one go.
//
// Everything runs on the thread that called runUI; other goroutines only PostMessage.

import (
	"math/bits"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"coregui/internal/canvas"
	"golang.org/x/sys/windows"
)

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	dwmapi  = windows.NewLazySystemDLL("dwmapi.dll")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pShowWindow               = user32.NewProc("ShowWindow")
	pSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	pAllowSetForegroundWindow = user32.NewProc("AllowSetForegroundWindow")
	pPostMessageW             = user32.NewProc("PostMessageW")
	pSendMessageW             = user32.NewProc("SendMessageW")
	pAdjustWindowRectEx       = user32.NewProc("AdjustWindowRectEx")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	pLoadImageW               = user32.NewProc("LoadImageW")
	pLoadCursorW              = user32.NewProc("LoadCursorW")
	pSetCursor                = user32.NewProc("SetCursor")
	pGetDpiForSystem          = user32.NewProc("GetDpiForSystem")
	pFindWindowW              = user32.NewProc("FindWindowW")
	pInvalidateRect           = user32.NewProc("InvalidateRect")
	pBeginPaint               = user32.NewProc("BeginPaint")
	pEndPaint                 = user32.NewProc("EndPaint")
	pGetClientRect            = user32.NewProc("GetClientRect")
	pClientToScreen           = user32.NewProc("ClientToScreen")
	pTrackMouseEvent          = user32.NewProc("TrackMouseEvent")
	pDrawTextW                = user32.NewProc("DrawTextW")
	pDrawIconEx               = user32.NewProc("DrawIconEx")
	pCreateFontW              = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush         = gdi32.NewProc("CreateSolidBrush")
	pCreateCompatibleDC       = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection         = gdi32.NewProc("CreateDIBSection")
	pSelectObject             = gdi32.NewProc("SelectObject")
	pDeleteObject             = gdi32.NewProc("DeleteObject")
	pBitBlt                   = gdi32.NewProc("BitBlt")
	pGdiFlush                 = gdi32.NewProc("GdiFlush")
	pSetTextColor             = gdi32.NewProc("SetTextColor")
	pSetBkMode                = gdi32.NewProc("SetBkMode")
	pShellExecuteW            = shell32.NewProc("ShellExecuteW")
	pDwmSetWindowAttribute    = dwmapi.NewProc("DwmSetWindowAttribute")
	pGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
)

const (
	windowClass = "CoreGUIWindow"
	creditName  = "fanti1"
	creditURL   = "https://github.com/fanti1"
	windowWidth = 380 // DIP

	wmPaint       = 0x000F
	wmClose       = 0x0010
	wmEraseBkgnd  = 0x0014
	wmSetCursor   = 0x0020
	wmSetIcon     = 0x0080
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmMouseLeave  = 0x02A3
	wmShow        = 0x8000 + 1 // WM_APP + 1
	wmRefresh     = 0x8000 + 2

	dtCenter   = 0x1
	dtRight    = 0x2
	dtVCenter  = 0x4
	dtWordWrap = 0x10
	dtSingle   = 0x20
	dtNoPrefix = 0x800
	dtEllipsis = 0x8000
	dtLine     = dtSingle | dtVCenter | dtNoPrefix | dtEllipsis
)

// Clickable areas.
const (
	hitNone = iota
	hitCore0
	hitCore01
	hitAutostart
	hitCloseToTray
	hitLanguage
	hitAbout
	hitBack
	hitLink
)

const (
	pageMain = iota
	pageAbout
)

// Dark palette.
var (
	cBg        = canvas.Hex(0x121417)
	cCard      = canvas.Hex(0x1a1e25)
	cBorder    = canvas.Hex(0x272c35)
	cHover     = canvas.Hex(0x232933)
	cText      = canvas.Hex(0xe8eaed)
	cMuted     = canvas.Hex(0x939aa5)
	cAccent    = canvas.Hex(0x2ed573)
	cAccentDim = canvas.Hex(0x1c4430)
	cPlanned   = canvas.Hex(0x1b2d24)
	cRed       = canvas.Hex(0xff5c5c)
	cRedDim    = canvas.Hex(0x41212a)
	cIdle      = canvas.Hex(0x6b7280)
	cTrackOff  = canvas.Hex(0x3a404a)
	cKnob      = canvas.Hex(0xf1f3f5)
	cSegBg     = canvas.Hex(0x14171c)
	cOnAccent  = canvas.Hex(0x0b0d10)
)

type winRect struct{ left, top, right, bottom int32 }

type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	iconSm                             uintptr
}

type winMsg struct {
	hwnd           uintptr
	message        uint32
	wParam, lParam uintptr
	time           uint32
	pt             [2]int32
	private        uint32
}

type paintStruct struct {
	hdc                uintptr
	erase              int32
	rc                 winRect
	restore, incUpdate int32
	reserved           [32]byte
}

type bitmapInfoHeader struct {
	size                   uint32
	width, height          int32
	planes, bitCount       uint16
	compression, sizeImage uint32
	xPPM, yPPM             int32
	clrUsed, clrImportant  uint32
}

type textOp struct {
	s     string
	r     winRect
	font  uintptr
	color canvas.Color
	flags uintptr
}

type hitArea struct {
	r  winRect
	id int
}

// settingsWindow holds the window state. Only the UI thread touches it.
type settingsWindow struct {
	hwnd, inst         uintptr
	dpi                int
	fTitle, fBody      uintptr
	fBold, fSmall      uintptr
	fSection, fCell    uintptr
	iconBig, iconSmall uintptr
	iconHeader         uintptr
	cursorHand         uintptr
	memDC, dib, oldBmp uintptr
	cv                 canvas.Canvas
	texts              []textOp
	hits               []hitArea
	hover, pressed     int
	tracking           bool
	page               int
}

var (
	win     settingsWindow
	winHwnd atomic.Uintptr // read by other goroutines to PostMessage
)

// call invokes a Win32 function. uintptrescapes keeps pointers converted in the
// arguments (e.g. uintptr(unsafe.Pointer(&x))) alive and pinned during the call.
//
//go:uintptrescapes
func call(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}

// UTF-16 strings handed to Win32 are cached forever, so the GC can never free a
// buffer Windows is still reading. The set is small: UI texts × languages.
var (
	strMu    sync.Mutex
	strCache = map[string]*uint16{}
)

func wstr(s string) *uint16 {
	strMu.Lock()
	defer strMu.Unlock()
	p, ok := strCache[s]
	if !ok {
		p, _ = windows.UTF16PtrFromString(s)
		strCache[s] = p
	}
	return p
}

func utf16(s string) uintptr { return uintptr(unsafe.Pointer(wstr(s))) }

func colorRef(c canvas.Color) uintptr { return uintptr(c.R) | uintptr(c.G)<<8 | uintptr(c.B)<<16 }

// ---------------------------------------------------------------- lifecycle

// runUI creates the window and the tray icon and runs the message loop on this
// thread until the user exits. show=false starts in the tray (used by autostart).
func runUI(show bool) {
	runtime.LockOSThread()
	enableDarkMenus()
	win.create()
	tray.init()
	tray.add()
	go run()
	if show {
		win.show()
	}
	var m winMsg
	for call(pGetMessageW, uintptr(unsafe.Pointer(&m)), 0, 0, 0) != 0 {
		call(pTranslateMessage, uintptr(unsafe.Pointer(&m)))
		call(pDispatchMessageW, uintptr(unsafe.Pointer(&m)))
	}
	tray.remove()
}

// quit ends the message loop (UI thread only).
func quit() { call(pPostQuitMessage, 0) }

// refreshUI asks the UI thread to redraw the window and the tray. Safe from any goroutine.
func refreshUI() {
	if h := winHwnd.Load(); h != 0 {
		call(pPostMessageW, h, wmRefresh, 0, 0)
	}
}

// showExistingInstance asks an already running instance to show its window.
func showExistingInstance() {
	if h := call(pFindWindowW, utf16(windowClass), 0); h != 0 {
		call(pAllowSetForegroundWindow, 0xFFFFFFFF) // ASFW_ANY
		call(pPostMessageW, h, wmShow, 0, 0)
	}
}

// enableDarkMenus makes popup menus (tray, language) dark. It uses uxtheme's
// SetPreferredAppMode / FlushMenuThemes (ordinals 135/136, Windows 10 1903+);
// on older systems it silently does nothing.
func enableDarkMenus() {
	h, err := windows.LoadLibraryEx("uxtheme.dll", 0, windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return
	}
	if p, err := windows.GetProcAddressByOrdinal(h, 135); err == nil {
		syscall.SyscallN(p, 2) // ForceDark
	}
	if p, err := windows.GetProcAddressByOrdinal(h, 136); err == nil {
		syscall.SyscallN(p)
	}
}

func (w *settingsWindow) px(v float64) int32   { return int32(v*float64(w.dpi)/96 + 0.5) }
func (w *settingsWindow) pf(v float64) float64 { return v * float64(w.dpi) / 96 }

func (w *settingsWindow) font(sizePx float64, weight int) uintptr {
	return call(pCreateFontW, uintptr(-w.px(sizePx)), 0, 0, 0, uintptr(weight),
		0, 0, 0, 1 /*DEFAULT_CHARSET*/, 0, 0, 5 /*CLEARTYPE_QUALITY*/, 0, utf16("Segoe UI"))
}

func (w *settingsWindow) create() {
	w.inst = call(pGetModuleHandleW, 0)
	w.dpi = 96
	if pGetDpiForSystem.Find() == nil {
		w.dpi = int(call(pGetDpiForSystem))
	}
	w.fTitle = w.font(20, 600)
	w.fBody = w.font(13, 400)
	w.fBold = w.font(13.5, 600)
	w.fSmall = w.font(11.5, 400)
	w.fSection = w.font(11, 700)
	w.fCell = w.font(10.5, 600)
	w.cursorHand = call(pLoadCursorW, 0, 32649) // IDC_HAND

	w.iconBig = call(pLoadImageW, w.inst, 1, 1 /*IMAGE_ICON*/, call(pGetSystemMetrics, 11), call(pGetSystemMetrics, 12), 0)
	w.iconSmall = call(pLoadImageW, w.inst, 1, 1, call(pGetSystemMetrics, 49), call(pGetSystemMetrics, 50), 0)
	w.iconHeader = call(pLoadImageW, w.inst, 1, 1, uintptr(w.px(34)), uintptr(w.px(34)), 0)

	wc := wndClassEx{
		wndProc:    windows.NewCallback(wndProc),
		instance:   w.inst,
		icon:       w.iconBig,
		iconSm:     w.iconSmall,
		cursor:     call(pLoadCursorW, 0, 32512 /*IDC_ARROW*/),
		background: call(pCreateSolidBrush, colorRef(cBg)),
		className:  wstr(windowClass),
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	call(pRegisterClassExW, uintptr(unsafe.Pointer(&wc)))

	const style = 0x00C00000 | 0x00080000 | 0x00020000 // WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX
	r := winRect{0, 0, w.px(windowWidth), w.px(mainHeight())}
	call(pAdjustWindowRectEx, uintptr(unsafe.Pointer(&r)), style, 0, 0)
	ww, wh := r.right-r.left, r.bottom-r.top
	x := (int32(call(pGetSystemMetrics, 0)) - ww) / 2
	y := (int32(call(pGetSystemMetrics, 1)) - wh) / 2
	w.hwnd = call(pCreateWindowExW, 0, utf16(windowClass), utf16(appName), style,
		uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), 0, 0, w.inst, 0)
	call(pSendMessageW, w.hwnd, wmSetIcon, 1, w.iconBig)
	call(pSendMessageW, w.hwnd, wmSetIcon, 0, w.iconSmall)

	dark := int32(1) // dark title bar: attribute 20 on Windows 11 / 10 20H1+, 19 on older 10
	if call(pDwmSetWindowAttribute, w.hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4) != 0 {
		call(pDwmSetWindowAttribute, w.hwnd, 19, uintptr(unsafe.Pointer(&dark)), 4)
	}
	winHwnd.Store(w.hwnd)
}

func (w *settingsWindow) show() {
	call(pShowWindow, w.hwnd, 1 /*SW_SHOWNORMAL*/)
	call(pSetForegroundWindow, w.hwnd)
}

func (w *settingsWindow) invalidate() { call(pInvalidateRect, w.hwnd, 0, 0) }

// ---------------------------------------------------------------- layout & drawing

// cpuList returns the logical CPUs of the system in order (nil if unknown).
func cpuList() []int {
	all, err := systemMask()
	if err != nil {
		return nil
	}
	var cpus []int
	for m := all; m != 0; m &= m - 1 {
		cpus = append(cpus, bits.TrailingZeros64(m))
	}
	return cpus
}

const gridCols = 16

func statusCardHeight() float64 {
	rows := (len(cpuList()) + gridCols - 1) / gridCols
	return 64 + float64(rows)*26 + 24
}

// mainHeight is the client height of the window in DIP.
func mainHeight() float64 { return 68 + statusCardHeight() + 14 + 58 + 16 + 22 + 132 + 14 + 18 + 14 }

func (w *settingsWindow) rect(x, y, wd, h float64) winRect {
	return winRect{w.px(x), w.px(y), w.px(x + wd), w.px(y + h)}
}

func (w *settingsWindow) shape(x0, y0, x1, y1, r float64, c canvas.Color) {
	w.cv.RoundRect(w.pf(x0), w.pf(y0), w.pf(x1), w.pf(y1), w.pf(r), c)
}

func (w *settingsWindow) card(x0, y0, x1, y1, r float64, border, fill canvas.Color) {
	w.cv.Border(w.pf(x0), w.pf(y0), w.pf(x1), w.pf(y1), w.pf(r), w.pf(1), border, fill)
}

func (w *settingsWindow) text(s string, x, y, wd, h float64, font uintptr, c canvas.Color, flags uintptr) {
	w.texts = append(w.texts, textOp{s, w.rect(x, y, wd, h), font, c, flags})
}

func (w *settingsWindow) hit(x, y, wd, h float64, id int) {
	w.hits = append(w.hits, hitArea{w.rect(x, y, wd, h), id})
}

func (w *settingsWindow) hitAt(x, y int32) int {
	for _, h := range w.hits {
		if x >= h.r.left && x < h.r.right && y >= h.r.top && y < h.r.bottom {
			return h.id
		}
	}
	return hitNone
}

// ensureBuffer (re)creates the off-screen DIB section matching the client size.
func (w *settingsWindow) ensureBuffer(hdc uintptr, width, height int) {
	if w.memDC != 0 && w.cv.W == width && w.cv.H == height {
		return
	}
	if w.memDC != 0 {
		call(pSelectObject, w.memDC, w.oldBmp)
		call(pDeleteObject, w.dib)
	} else {
		w.memDC = call(pCreateCompatibleDC, hdc)
	}
	bi := bitmapInfoHeader{size: 40, width: int32(width), height: -int32(height), planes: 1, bitCount: 32}
	var pix unsafe.Pointer
	w.dib = call(pCreateDIBSection, hdc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&pix)), 0, 0)
	w.oldBmp = call(pSelectObject, w.memDC, w.dib)
	w.cv = canvas.Canvas{Pix: unsafe.Slice((*byte)(pix), width*height*4), W: width, H: height, Stride: width * 4}
}

func (w *settingsWindow) paint() {
	var ps paintStruct
	hdc := call(pBeginPaint, w.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer call(pEndPaint, w.hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc winRect
	call(pGetClientRect, w.hwnd, uintptr(unsafe.Pointer(&rc)))
	width, height := int(rc.right), int(rc.bottom)
	if width <= 0 || height <= 0 {
		return
	}
	w.ensureBuffer(hdc, width, height)
	call(pGdiFlush) // GDI must be done with the buffer before Go writes to it

	w.texts, w.hits = w.texts[:0], w.hits[:0]
	w.cv.Fill(cBg)
	w.drawHeader()
	if w.page == pageAbout {
		w.drawAbout(float64(height) * 96 / float64(w.dpi))
	} else {
		w.drawMain()
	}

	// GDI pass: icon and text on top of the shapes.
	call(pDrawIconEx, w.memDC, uintptr(w.px(18)), uintptr(w.px(17)), w.iconHeader, uintptr(w.px(34)), uintptr(w.px(34)), 0, 0, 3)
	call(pSetBkMode, w.memDC, 1) // TRANSPARENT
	for _, t := range w.texts {
		call(pSelectObject, w.memDC, t.font)
		call(pSetTextColor, w.memDC, colorRef(t.color))
		r := t.r
		call(pDrawTextW, w.memDC, utf16(t.s), ^uintptr(0), uintptr(unsafe.Pointer(&r)), t.flags)
	}
	call(pBitBlt, hdc, 0, 0, uintptr(width), uintptr(height), w.memDC, 0, 0, 0x00CC0020 /*SRCCOPY*/)
}

func (w *settingsWindow) drawHeader() {
	w.text(appName, 62, 13, 300, 26, w.fTitle, cText, dtLine)
	w.text(T("tagline"), 62, 38, 300, 18, w.fBody, cMuted, dtLine)
}

func (w *settingsWindow) drawMain() {
	s := currentStatus()
	ex := exclude.Load()

	// Status card: state, detail line, CPU grid and legend.
	y := 68.0
	cardH := statusCardHeight()
	w.card(16, y, 364, y+cardH, 10, cBorder, cCard)
	dot := map[state]canvas.Color{stateIdle: cIdle, stateOK: cAccent, stateError: cRed}[s.state]
	w.cv.Circle(w.pf(33), w.pf(y+24), w.pf(5), dot)
	main, detail := statusLines(s)
	if detail == "" {
		detail = plannedSummary()
	}
	main = strings.TrimLeft(main, "✔✖ ") // the colored dot already shows the state
	w.text(main, 46, y+13, 304, 22, w.fBold, cText, dtLine)
	w.text(detail, 28, y+36, 322, 18, w.fBody, cMuted, dtLine)

	cpus := cpuList()
	cols := min(len(cpus), gridCols)
	const gap = 4.0
	cw := (320 - gap*float64(cols-1)) / float64(max(cols, 1))
	for k, cpu := range cpus {
		x := 30 + float64(k%gridCols)*(cw+gap)
		cy := y + 62 + float64(k/gridCols)*26
		fill, fg := cPlanned, cAccent // idle: planned use
		switch {
		case ex&(1<<cpu) != 0:
			fill, fg = cRedDim, cRed
		case s.state == stateOK && s.mask&(1<<cpu) != 0:
			fill, fg = cAccentDim, cAccent
		case s.state == stateError:
			fill, fg = cHover, cMuted
		}
		w.shape(x, cy, x+cw, cy+22, 4, fill)
		w.text(strconv.Itoa(cpu), x, cy, cw, 22, w.fCell, fg, dtLine|dtCenter)
	}
	ly := y + cardH - 26
	w.shape(30, ly+5, 40, ly+15, 2, cAccent)
	w.text(T("legend_used"), 46, ly, 140, 20, w.fSmall, cMuted, dtLine)
	w.shape(190, ly+5, 200, ly+15, 2, cRed)
	w.text(T("legend_removed"), 206, ly, 140, 20, w.fSmall, cMuted, dtLine)

	// Segmented control: Core 0 | Core 0 and 1.
	y += cardH + 14
	w.text(strings.ToUpper(T("remove_group")), 18, y, 330, 16, w.fSection, cMuted, dtLine)
	y += 22
	w.card(16, y, 364, y+36, 9, cBorder, cSegBg)
	for i, opt := range []struct {
		id   int
		mask uint64
		key  string
	}{{hitCore0, excludeCore0, "core0"}, {hitCore01, excludeCore01, "core01"}} {
		x0 := 16 + float64(i)*174
		fg := cText
		switch {
		case ex == opt.mask:
			w.shape(x0+4, y+4, x0+170, y+32, 6, cAccent)
			fg = cOnAccent
		case w.hover == opt.id:
			w.shape(x0+4, y+4, x0+170, y+32, 6, cHover)
		}
		w.text(T(opt.key), x0, y, 174, 36, w.fBold, fg, dtLine|dtCenter)
		w.hit(x0, y, 174, 36, opt.id)
	}

	// Options card: two toggles and the language picker.
	y += 36 + 16
	w.text(strings.ToUpper(T("options")), 18, y, 330, 16, w.fSection, cMuted, dtLine)
	y += 22
	w.card(16, y, 364, y+132, 10, cBorder, cCard)
	rows := []struct {
		id    int
		label string
	}{{hitAutostart, T("autostart")}, {hitCloseToTray, T("close_tray")}, {hitLanguage, T("language")}}
	for i, row := range rows {
		ry := y + float64(i)*44
		if w.hover == row.id {
			w.shape(17, ry+1, 363, ry+43, 9, cHover)
		}
		if i > 0 {
			w.shape(28, ry, 352, ry+1, 0, cBorder)
		}
		w.text(row.label, 30, ry, 220, 44, w.fBody, cText, dtLine)
		w.hit(16, ry, 348, 44, row.id)
	}
	w.toggle(y, autostartEnabled())
	w.toggle(y+44, closeToTray.Load())
	w.text(languageLabel()+"  ▾", 150, y+88, 200, 44, w.fBody, cMuted, dtLine|dtRight)

	// Footer: About link and credit.
	y += 132 + 14
	aboutColor := cMuted
	if w.hover == hitAbout {
		aboutColor = cAccent
	}
	w.text(T("about"), 18, y, 150, 18, w.fBody, aboutColor, dtLine)
	w.hit(16, y-4, 120, 26, hitAbout)
	w.text("v"+appVersion+" · "+creditName, 170, y, 194, 18, w.fSmall, cMuted, dtLine|dtRight)
}

// toggle draws a switch at the right edge of the options row starting at rowY.
func (w *settingsWindow) toggle(rowY float64, on bool) {
	x, y := 364-16-42.0, rowY+11
	track, knobX := cTrackOff, x+11
	if on {
		track, knobX = cAccent, x+31
	}
	w.shape(x, y, x+42, y+22, 11, track)
	w.cv.Circle(w.pf(knobX), w.pf(y+11), w.pf(8), cKnob)
}

func (w *settingsWindow) drawAbout(height float64) {
	y := 68.0
	w.card(16, y, 364, height-70, 10, cBorder, cCard)
	w.text(T("about"), 32, y+14, 300, 22, w.fBold, cText, dtLine)
	w.text(T("about_text"), 32, y+42, 316, 110, w.fBody, cMuted, dtWordWrap|dtNoPrefix)
	ly := y + 160
	for _, line := range []string{Tf("version", appVersion), T("license"), Tf("created_by", creditName)} {
		w.text(line, 32, ly, 316, 22, w.fBody, cText, dtLine)
		ly += 24
	}
	link := cAccent
	if w.hover == hitLink {
		link = cKnob
	}
	w.text("github.com/"+creditName, 32, ly, 316, 22, w.fBody, link, dtLine)
	w.hit(30, ly, 160, 22, hitLink)

	by := height - 56
	fill := cCard
	if w.hover == hitBack {
		fill = cHover
	}
	w.card(16, by, 364, by+38, 9, cBorder, fill)
	w.text("← "+T("back"), 16, by, 348, 38, w.fBold, cText, dtLine|dtCenter)
	w.hit(16, by, 348, 38, hitBack)
}

func languageLabel() string {
	if languageSetting() == "auto" {
		return T("auto") + " (" + current.Load().name + ")"
	}
	return current.Load().name
}

// ---------------------------------------------------------------- input

func (w *settingsWindow) click(id int) {
	switch id {
	case hitCore0:
		go setExclude(excludeCore0)
	case hitCore01:
		go setExclude(excludeCore01)
	case hitAutostart:
		go toggleAutostart(!autostartEnabled())
	case hitCloseToTray:
		go setCloseToTray(!closeToTray.Load())
	case hitLanguage:
		w.languageMenu()
	case hitAbout:
		w.page, w.hover = pageAbout, hitNone
	case hitBack:
		w.page, w.hover = pageMain, hitNone
	case hitLink:
		call(pShellExecuteW, w.hwnd, utf16("open"), utf16(creditURL), 0, 0, 1)
	}
	w.invalidate()
}

// languageMenu pops up the language list under the language row.
func (w *settingsWindow) languageMenu() {
	const base = 1000
	check := func(on bool) uintptr {
		if on {
			return mfChecked
		}
		return mfString
	}
	m := call(pCreatePopupMenu)
	defer call(pDestroyMenu, m)
	call(pAppendMenuW, m, check(languageSetting() == "auto"), base, utf16(T("auto")))
	call(pAppendMenuW, m, mfSeparator, 0, 0)
	for i, l := range languages {
		call(pAppendMenuW, m, check(languageSetting() == l.code), uintptr(base+1+i), utf16(l.name))
	}
	for _, h := range w.hits {
		if h.id != hitLanguage {
			continue
		}
		pt := [2]int32{h.r.right - w.px(160), h.r.bottom}
		call(pClientToScreen, w.hwnd, uintptr(unsafe.Pointer(&pt)))
		switch cmd := int(call(pTrackPopupMenu, m, tpmRetCmd, uintptr(pt[0]), uintptr(pt[1]), 0, w.hwnd, 0)); {
		case cmd == base:
			go setLanguage("auto")
		case cmd > base && cmd <= base+len(languages):
			go setLanguage(languages[cmd-base-1].code)
		}
	}
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	w := &win
	x, y := int32(int16(lParam)), int32(int16(lParam>>16))
	switch msg {
	case wmPaint:
		w.paint()
		return 0

	case wmEraseBkgnd:
		return 1 // everything is painted in wmPaint (no flicker)

	case wmMouseMove:
		if !w.tracking { // ask for wmMouseLeave to clear the hover state
			tme := struct {
				size, flags uint32
				hwnd        uintptr
				hover       uint32
			}{24, 2 /*TME_LEAVE*/, hwnd, 0}
			call(pTrackMouseEvent, uintptr(unsafe.Pointer(&tme)))
			w.tracking = true
		}
		if id := w.hitAt(x, y); id != w.hover {
			w.hover = id
			w.invalidate()
		}
		return 0

	case wmMouseLeave:
		w.tracking = false
		if w.hover != hitNone {
			w.hover = hitNone
			w.invalidate()
		}
		return 0

	case wmLButtonDown:
		w.pressed = w.hitAt(x, y)
		return 0

	case wmLButtonUp:
		if id := w.hitAt(x, y); id != hitNone && id == w.pressed {
			w.click(id)
		}
		w.pressed = hitNone
		return 0

	case wmSetCursor:
		if lParam&0xFFFF == 1 /*HTCLIENT*/ && w.hover != hitNone {
			call(pSetCursor, w.cursorHand)
			return 1
		}

	case wmClose:
		if closeToTray.Load() {
			call(pShowWindow, hwnd, 0) // SW_HIDE: keep running in the tray
			w.page = pageMain
		} else {
			quit()
		}
		return 0

	case wmShow:
		w.show()
		return 0

	case wmRefresh:
		w.invalidate()
		tray.update()
		return 0

	case wmTray:
		switch lParam & 0xFFFF {
		case wmLButtonUp:
			w.show()
		case wmRButtonUp, wmContextMenu:
			tray.menu()
		}
		return 0

	case tray.taskbarCreated: // Explorer restarted: the icon must be added again
		if msg != 0 {
			tray.add()
			return 0
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}
