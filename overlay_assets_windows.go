// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"math"
)

func loadControllerArtwork(path string) (controllerArtwork, bool) {
	f, e := embeddedAssets.Open(path)
	if e != nil {
		return controllerArtwork{}, false
	}
	img, _, e := image.Decode(f)
	f.Close()
	if e != nil {
		return controllerArtwork{}, false
	}
	b := img.Bounds()
	canvas := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(canvas, canvas.Bounds(), img, b.Min, draw.Src)
	return artworkFromNRGBA(canvas)
}

func loadUIIconMask(path string) (uiIconMask, bool) {
	f, err := embeddedAssets.Open(path)
	if err != nil {
		return uiIconMask{}, false
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return uiIconMask{}, false
	}
	bounds := img.Bounds()
	canvas := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), img, bounds.Min, draw.Src)
	if canvas.Bounds().Dx() <= 0 || canvas.Bounds().Dy() <= 0 {
		return uiIconMask{}, false
	}
	alpha := make([]byte, canvas.Bounds().Dx()*canvas.Bounds().Dy())
	visible := false
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			a := canvas.NRGBAAt(x, y).A
			alpha[y*canvas.Bounds().Dx()+x] = a
			visible = visible || a != 0
		}
	}
	if !visible {
		return uiIconMask{}, false
	}
	return uiIconMask{w: int32(canvas.Bounds().Dx()), h: int32(canvas.Bounds().Dy()), alpha: alpha}, true
}

func makeUIIconArtwork(mask uiIconMask, boxSize int32) (controllerArtwork, bool) {
	return makeUIIconArtworkTinted(mask, boxSize, rgb(255, 255, 255))
}

func makeUIIconArtworkTinted(mask uiIconMask, boxSize int32, color uintptr) (controllerArtwork, bool) {
	if mask.w <= 0 || mask.h <= 0 || boxSize <= 0 || len(mask.alpha) < int(mask.w*mask.h) {
		return controllerArtwork{}, false
	}
	w, h := boxSize, boxSize
	if mask.w > mask.h {
		h = int32(math.Round(float64(boxSize) * float64(mask.h) / float64(mask.w)))
	} else {
		w = int32(math.Round(float64(boxSize) * float64(mask.w) / float64(mask.h)))
	}
	w, h = int32(clamp(int(w), 1, int(boxSize))), int32(clamp(int(h), 1, int(boxSize)))
	pixels := make([]byte, w*h*4)
	colorValue := uint32(color)
	red := uint32(byte(colorValue))
	green := uint32(byte(colorValue >> 8))
	blue := uint32(byte(colorValue >> 16))
	const samples = 4
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			sum := 0
			for sy := int32(0); sy < samples; sy++ {
				sourceY := ((y*samples + sy) * mask.h) / (h * samples)
				if sourceY >= mask.h {
					sourceY = mask.h - 1
				}
				for sx := int32(0); sx < samples; sx++ {
					sourceX := ((x*samples + sx) * mask.w) / (w * samples)
					if sourceX >= mask.w {
						sourceX = mask.w - 1
					}
					sum += int(mask.alpha[sourceY*mask.w+sourceX])
				}
			}
			a := byte((sum + samples*samples/2) / (samples * samples))
			i := int((y*w + x) * 4)
			// O desenho é uma máscara recolorível. BGRA pré-multiplicado mantém os
			// ícones personalizados idênticos aos glifos Fluent no estado ativo.
			alpha := uint32(a)
			pixels[i] = byte((blue * alpha) / 255)
			pixels[i+1] = byte((green * alpha) / 255)
			pixels[i+2] = byte((red * alpha) / 255)
			pixels[i+3] = a
		}
	}
	return controllerArtwork{w: w, h: h, pixels: pixels}, true
}

func drawUIIcon(_ uintptr, name string, r rect, color uintptr) bool {
	mask, ok := uiIconMasks[name]
	if !ok {
		return false
	}
	boxSize := scaled(20)
	key := fmt.Sprintf("%s:%d:%d", name, boxSize, color)
	artwork, ok := uiIconArtworkCache[key]
	if !ok {
		artwork, ok = makeUIIconArtworkTinted(mask, boxSize, color)
		if !ok {
			return false
		}
		uiIconArtworkCache[key] = artwork
	}
	drawArtworkBitmap(0, r, artwork)
	return true
}

func artworkFromNRGBA(canvas *image.NRGBA) (controllerArtwork, bool) {
	w, h := canvas.Bounds().Dx(), canvas.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	premul := make([]byte, w*h*4)
	k := 0
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			p := canvas.NRGBAAt(xx, yy)
			af := uint32(p.A)
			// UpdateLayeredWindow usa BGRA pré-multiplicado.
			b := byte((uint32(p.B) * af) / 255)
			g := byte((uint32(p.G) * af) / 255)
			r := byte((uint32(p.R) * af) / 255)
			premul[k], premul[k+1], premul[k+2], premul[k+3] = b, g, r, p.A
			k += 4
		}
	}
	return controllerArtwork{w: int32(w), h: int32(h), pixels: premul}, true
}

func initControllerArtwork() {
	for name, path := range map[string]string{
		gameBarIcon: "assets/icon_gamebar.png",
		usbIcon:     "assets/icon_usb.png",
	} {
		if mask, ok := loadUIIconMask(path); ok {
			uiIconMasks[name] = mask
		}
	}

	paths := []string{
		"assets/controller_ui_720.png",
		"assets/controller_ui_900.png",
		"assets/controller_ui_1080.png",
		"assets/controller_ui_1200.png",
		"assets/controller_ui_1440.png",
		"assets/controller_ui_1600.png",
		"assets/controller_ui_2160.png",
	}
	for _, path := range paths {
		if artwork, ok := loadControllerArtwork(path); ok {
			controllerArtworks = append(controllerArtworks, artwork)
		}
	}

	batteryPaths := []string{
		"assets/controller_ui_battery_720.png",
		"assets/controller_ui_battery_900.png",
		"assets/controller_ui_battery_1080.png",
		"assets/controller_ui_battery_1200.png",
		"assets/controller_ui_battery_1440.png",
		"assets/controller_ui_battery_1600.png",
		"assets/controller_ui_battery_2160.png",
	}
	for _, path := range batteryPaths {
		if artwork, ok := loadControllerArtwork(path); ok {
			batteryControllerArtworks = append(batteryControllerArtworks, artwork)
		}
	}
}

func clearScaleDependentArtworkCaches() {
	for key := range selectionFrameCache {
		delete(selectionFrameCache, key)
	}
	for key := range panelSurfaceCache {
		delete(panelSurfaceCache, key)
	}
	for key := range uiIconArtworkCache {
		delete(uiIconArtworkCache, key)
	}
	releaseCachedGDIObjects()
}

func shutdownControllerArtwork() {
	releaseLayeredFrame()
	controllerArtworks = nil
	batteryControllerArtworks = nil
	uiIconMasks = map[string]uiIconMask{}
	uiIconArtworkCache = map[string]controllerArtwork{}
	for key, iconHandle := range gameIconCache {
		if iconHandle != 0 {
			pDestroyIcon.Call(iconHandle)
		}
		delete(gameIconCache, key)
	}
	clearScaleDependentArtworkCaches()
}

func drawArtwork(hdc uintptr, r rect, artworks []controllerArtwork) {
	if len(artworks) == 0 {
		return
	}
	targetW := r.Right - r.Left
	targetH := r.Bottom - r.Top
	best := -1
	bestScore := int64(1<<62 - 1)
	for i, artwork := range artworks {
		if artwork.w > targetW || artwork.h > targetH {
			continue
		}
		dw := int64(targetW - artwork.w)
		dh := int64(targetH - artwork.h)
		score := dw*dw + dh*dh
		if score < bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		// Só acontece em uma resolução muito pequena: escolhe a menor arte.
		best = 0
		for i := range artworks {
			if artworks[i].w < artworks[best].w {
				best = i
			}
		}
	}
	// Desenha 1:1. O redimensionamento de alta qualidade já foi feito nos PNGs,
	// evitando a pixelização ao reduzir a imagem original durante a composição.
	drawArtworkBitmap(hdc, r, artworks[best])
}

func drawArtworkBitmap(_ uintptr, r rect, artwork controllerArtwork) {
	if artwork.w <= 0 || artwork.h <= 0 {
		return
	}
	targetW := r.Right - r.Left
	targetH := r.Bottom - r.Top
	x := r.Left + (targetW-artwork.w)/2
	y := r.Top + (targetH-artwork.h)/2

	if activeLayeredFrame == nil || len(artwork.pixels) < int(artwork.w*artwork.h*4) {
		return
	}
	composeArtworkPixels(activeLayeredFrame, x, y, artwork)
}

func composeArtworkPixels(frame *layeredFrame, x, y int32, artwork controllerArtwork) {
	if frame == nil {
		return
	}
	sourceLeft := int32(0)
	sourceTop := int32(0)
	sourceRight := artwork.w
	sourceBottom := artwork.h
	if x < 0 {
		sourceLeft = -x
	}
	if y < 0 {
		sourceTop = -y
	}
	if x+sourceRight > frame.width {
		sourceRight = frame.width - x
	}
	if y+sourceBottom > frame.height {
		sourceBottom = frame.height - y
	}
	if sourceLeft >= sourceRight || sourceTop >= sourceBottom {
		return
	}

	for sy := sourceTop; sy < sourceBottom; sy++ {
		si := int((sy*artwork.w + sourceLeft) * 4)
		di := int((y+sy)*frame.stride + (x+sourceLeft)*4)
		for sx := sourceLeft; sx < sourceRight; sx++ {
			sa := uint32(artwork.pixels[si+3])
			if sa == 0 {
				si += 4
				di += 4
				continue
			}
			if sa == 255 {
				frame.pixels[di] = artwork.pixels[si]
				frame.pixels[di+1] = artwork.pixels[si+1]
				frame.pixels[di+2] = artwork.pixels[si+2]
				frame.pixels[di+3] = 255
				si += 4
				di += 4
				continue
			}
			da := uint32(frame.pixels[di+3])
			inv := uint32(255) - sa
			db, dg, dr := uint32(0), uint32(0), uint32(0)
			if da != 0 {
				db = uint32(frame.pixels[di])
				dg = uint32(frame.pixels[di+1])
				dr = uint32(frame.pixels[di+2])
			}
			outB := uint32(artwork.pixels[si]) + (db*inv+127)/255
			outG := uint32(artwork.pixels[si+1]) + (dg*inv+127)/255
			outR := uint32(artwork.pixels[si+2]) + (dr*inv+127)/255
			outA := sa + (da*inv+127)/255
			if outB > 255 {
				outB = 255
			}
			if outG > 255 {
				outG = 255
			}
			if outR > 255 {
				outR = 255
			}
			if outA > 255 {
				outA = 255
			}
			frame.pixels[di] = byte(outB)
			frame.pixels[di+1] = byte(outG)
			frame.pixels[di+2] = byte(outR)
			frame.pixels[di+3] = byte(outA)
			si += 4
			di += 4
		}
	}
}

func drawController(hdc uintptr, r rect) {
	drawArtwork(hdc, r, controllerArtworks)
}

func drawBatteryController(hdc uintptr, r rect) {
	if len(batteryControllerArtworks) == 0 {
		iconColor(hdc, "\uE7FC", r, scaled(26), rgb(248, 248, 250))
		return
	}
	drawArtwork(hdc, r, batteryControllerArtworks)
}
