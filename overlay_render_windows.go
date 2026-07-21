// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"unsafe"
)

func paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	renderLayeredOverlay(hdc, hwnd)
}

func renderLayeredOverlay(hdc uintptr, hwnd uintptr) {
	var c rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&c)))
	w := c.Right - c.Left
	h := c.Bottom - c.Top
	if w <= 0 || h <= 0 {
		return
	}

	frame := acquireLayeredFrame(hdc, w, h)
	if frame == nil {
		return
	}
	// clear usa o caminho otimizado do runtime para zerar toda a DIB. O código
	// anterior gravava quatro bytes por pixel em Go e custava caro em 4K.
	clear(frame.pixels)
	activeLayeredFrame = frame
	defer func() { activeLayeredFrame = nil }()

	pSetBkMode.Call(frame.hdc, TRANSPARENT)
	state.mu.Lock()
	mode := state.overlayMode
	state.mu.Unlock()
	if mode == overlayMenu {
		paintMenu(frame.hdc, c)
	} else if mode == overlayBattery {
		paintBattery(frame.hdc, c)
	} else if mode == overlayMessage {
		paintMessage(frame.hdc, c)
	}

	finalizeLayeredFrame(frame)

	var wr rect
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	dst := point{X: wr.Left, Y: wr.Top}
	sz := size{CX: w, CY: h}
	src := point{}
	blend := blendFunction{BlendOp: AC_SRC_OVER, SourceConstantAlpha: 255, AlphaFormat: AC_SRC_ALPHA}
	pUpdateLayeredWindow.Call(
		hwnd,
		0,
		uintptr(unsafe.Pointer(&dst)),
		uintptr(unsafe.Pointer(&sz)),
		frame.hdc,
		uintptr(unsafe.Pointer(&src)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ULW_ALPHA,
	)
}

func acquireLayeredFrame(referenceDC uintptr, w, h int32) *layeredFrame {
	if cachedLayeredFrame != nil && cachedLayeredFrame.width == w && cachedLayeredFrame.height == h {
		return cachedLayeredFrame
	}
	releaseLayeredFrame()

	mem, _, _ := pCreateCompatibleDC.Call(referenceDC)
	if mem == 0 {
		return nil
	}
	bmi := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       w,
		Height:      -h,
		Planes:      1,
		BitCount:    32,
		Compression: BI_RGB,
		SizeImage:   uint32(w * h * 4),
	}}
	var bits unsafe.Pointer
	bmp, _, _ := pCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		if bmp != 0 {
			pDeleteObject.Call(bmp)
		}
		pDeleteDC.Call(mem)
		return nil
	}
	old, _, _ := pSelectObject.Call(mem, bmp)
	frame := &layeredFrame{
		bits:      bits,
		pixels:    unsafe.Slice((*byte)(bits), int(w*h*4)),
		width:     w,
		height:    h,
		stride:    w * 4,
		hdc:       mem,
		bitmap:    bmp,
		oldBitmap: old,
	}
	cachedLayeredFrame = frame
	return frame
}

func releaseLayeredFrame() {
	frame := cachedLayeredFrame
	if frame == nil {
		return
	}
	cachedLayeredFrame = nil
	if frame.hdc != 0 && frame.oldBitmap != 0 {
		pSelectObject.Call(frame.hdc, frame.oldBitmap)
	}
	if frame.bitmap != 0 {
		pDeleteObject.Call(frame.bitmap)
	}
	if frame.hdc != 0 {
		pDeleteDC.Call(frame.hdc)
	}
}

func finalizeLayeredFrame(frame *layeredFrame) {
	if frame == nil {
		return
	}
	p := frame.pixels
	for i := 0; i+3 < len(p); i += 4 {
		if p[i+3] != 0 {
			continue
		}
		// O frame foi zerado antes da pintura. Pixels RGB ainda zerados continuam
		// transparentes; desenhos GDI não nulos, que não escrevem alpha, são opacos.
		if p[i]|p[i+1]|p[i+2] == 0 {
			continue
		}
		p[i+3] = 255
	}
}

func fillRect(hdc uintptr, r rect, color uintptr) {
	b := cachedBrush(color)
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), b)
}

// fillOpaqueRect mantém o alpha da área explicitamente opaco no frame layered.
// Isso é necessário antes de compor halos semitransparentes: o GDI altera o RGB
// do DIB, mas pode deixar o canal alpha zerado até a finalização do frame. Se o
// efeito for composto nesse intervalo, surgem faixas translúcidas mostrando a área
// de trabalho entre as opções do mini menu.
func fillOpaqueRect(hdc uintptr, r rect, color uintptr) {
	fillRect(hdc, r, color)
	frame := activeLayeredFrame
	if frame == nil {
		return
	}
	left := clamp32(r.Left, 0, frame.width)
	top := clamp32(r.Top, 0, frame.height)
	right := clamp32(r.Right, 0, frame.width)
	bottom := clamp32(r.Bottom, 0, frame.height)
	if right <= left || bottom <= top {
		return
	}
	c := uint32(color)
	b := byte(c >> 16)
	g := byte(c >> 8)
	rr := byte(c)
	for y := top; y < bottom; y++ {
		row := int(y * frame.stride)
		for x := left; x < right; x++ {
			i := row + int(x*4)
			frame.pixels[i] = b
			frame.pixels[i+1] = g
			frame.pixels[i+2] = rr
			frame.pixels[i+3] = 255
		}
	}
}

func clamp32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func rounded(hdc uintptr, r rect, fill, border uintptr, radius int32, width int32) {
	if width <= 1 {
		width = scaledStroke(1)
	}
	br := cachedBrush(fill)
	pen := cachedPen(width, border)
	ob, _, _ := pSelectObject.Call(hdc, br)
	op, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(int64(r.Left)), uintptr(int64(r.Top)), uintptr(int64(r.Right)), uintptr(int64(r.Bottom)), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, ob)
	pSelectObject.Call(hdc, op)
}

func roundedOutline(hdc uintptr, r rect, border uintptr, radius int32, width int32) {
	if width <= 1 {
		width = scaledStroke(1)
	}
	pen := cachedPen(width, border)
	hollow, _, _ := pGetStockObject.Call(5) // HOLLOW_BRUSH
	oldBrush, _, _ := pSelectObject.Call(hdc, hollow)
	oldPen, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(int64(r.Left)), uintptr(int64(r.Top)), uintptr(int64(r.Right)), uintptr(int64(r.Bottom)), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldBrush)
	pSelectObject.Call(hdc, oldPen)
}

func makeFont(px int32, weight int32, name string) uintptr {
	return cachedFont(px, weight, name)
}

func measureTextWidth(s string, px, weight int32, fontName string) int32 {
	if s == "" || px <= 0 {
		return 0
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 0
	}
	defer pReleaseDC.Call(0, hdc)

	f := makeFont(px, weight, fontName)
	if f == 0 {
		return 0
	}
	old, _, _ := pSelectObject.Call(hdc, f)
	defer pSelectObject.Call(hdc, old)

	r := rect{}
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(utf16Ptr(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CALCRECT|DT_SINGLELINE)
	return r.Right - r.Left
}

func textFont(hdc uintptr, s string, r rect, color uintptr, px, weight int32, fontName string, flags uint32) {
	f := makeFont(px, weight, fontName)
	if f == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(hdc, f)
	pSetTextColor.Call(hdc, color)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(utf16Ptr(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags))
	pSelectObject.Call(hdc, old)
}

func text(hdc uintptr, s string, r rect, color uintptr, px, weight int32, flags uint32) {
	textFont(hdc, s, r, color, px, weight, "Segoe UI Variable Text", flags)
}

func icon(hdc uintptr, glyph string, r rect, px int32) {
	iconColor(hdc, glyph, r, px, rgb(245, 245, 248))
}

func iconColor(hdc uintptr, glyph string, r rect, px int32, color uintptr) {
	f := makeFont(px, FW_NORMAL, "Segoe Fluent Icons")
	if f == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(hdc, f)
	pSetTextColor.Call(hdc, color)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(utf16Ptr(glyph))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	pSelectObject.Call(hdc, old)
}

func line(hdc uintptr, x1, y1, x2, y2 int32, color uintptr) {
	pen := cachedPen(scaledStroke(1), color)
	old, _, _ := pSelectObject.Call(hdc, pen)
	pMoveToEx.Call(hdc, uintptr(int64(x1)), uintptr(int64(y1)), 0)
	pLineTo.Call(hdc, uintptr(int64(x2)), uintptr(int64(y2)))
	pSelectObject.Call(hdc, old)
}
