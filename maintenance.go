//go:build windows

package main

import (
	"unsafe"
)

type gdiFontKey struct {
	px     int32
	weight int32
	name   string
}

type gdiPenKey struct {
	width int32
	color uintptr
}

var (
	gdiFontCache  = map[gdiFontKey]uintptr{}
	gdiBrushCache = map[uintptr]uintptr{}
	gdiPenCache   = map[gdiPenKey]uintptr{}
)

func cachedFont(px, weight int32, name string) uintptr {
	key := gdiFontKey{px: px, weight: weight, name: name}
	if font := gdiFontCache[key]; font != 0 {
		return font
	}
	font, _, _ := pCreateFontW.Call(
		uintptr(int64(-px)), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr(name))),
	)
	if font != 0 {
		gdiFontCache[key] = font
	}
	return font
}

func cachedBrush(color uintptr) uintptr {
	if brush := gdiBrushCache[color]; brush != 0 {
		return brush
	}
	brush, _, _ := pCreateSolidBrush.Call(color)
	if brush != 0 {
		gdiBrushCache[color] = brush
	}
	return brush
}

func cachedPen(width int32, color uintptr) uintptr {
	key := gdiPenKey{width: width, color: color}
	if pen := gdiPenCache[key]; pen != 0 {
		return pen
	}
	pen, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), color)
	if pen != 0 {
		gdiPenCache[key] = pen
	}
	return pen
}

func releaseCachedGDIObjects() {
	for key, font := range gdiFontCache {
		if font != 0 {
			pDeleteObject.Call(font)
		}
		delete(gdiFontCache, key)
	}
	for key, brush := range gdiBrushCache {
		if brush != 0 {
			pDeleteObject.Call(brush)
		}
		delete(gdiBrushCache, key)
	}
	for key, pen := range gdiPenCache {
		if pen != 0 {
			pDeleteObject.Call(pen)
		}
		delete(gdiPenCache, key)
	}
}
