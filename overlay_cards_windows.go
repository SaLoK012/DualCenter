// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"fmt"
	"image"
	"math"
	"strings"
)

func drawPS5Card(hdc uintptr, r rect) {
	// Painel grafite com borda azul controlada, inspirado no menu do PS5.
	rounded(hdc, r, rgb(12, 15, 20), rgb(24, 65, 104), scaled(20), scaledStroke(3))
	inset := scaled(3)
	rounded(hdc, rect{r.Left + inset, r.Top + inset, r.Right - inset, r.Bottom - inset}, rgb(12, 15, 20), rgb(42, 148, 238), scaled(18), scaledStroke(1))
}

func batteryIconArtwork(pct int, known bool, w, h int32) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	pct = clamp(pct, 0, 100)
	key := fmt.Sprintf("battery:%d:%d:%d:%t:%d", w, h, pct, known, int(math.Round(state.scale*1000)))
	if artwork, ok := selectionFrameCache[key]; ok {
		return artwork, true
	}

	capW := scaled(3)
	if capW < 2 {
		capW = 2
	}
	aw := w + capW
	canvas := image.NewNRGBA(image.Rect(0, 0, int(aw), int(h)))

	const samples = 4
	totalSamples := samples * samples
	bodyInset := math.Max(0.65, 0.55*state.scale)
	bodyCX := bodyInset + (float64(w)-2*bodyInset)/2
	bodyCY := bodyInset + (float64(h)-2*bodyInset)/2
	bodyHalfW := (float64(w) - 2*bodyInset) / 2
	bodyHalfH := (float64(h) - 2*bodyInset) / 2
	bodyRadius := math.Min(float64(scaled(3)), bodyHalfH)
	stroke := math.Max(1.05, 1.15*state.scale)

	innerLeft := float64(scaled(3))
	innerRight := float64(w - scaled(3))
	innerTop := float64(scaled(3))
	innerBottom := float64(h - scaled(3))
	fillRight := innerLeft
	if known && innerRight > innerLeft {
		fillRight = innerLeft + (innerRight-innerLeft)*float64(pct)/100.0
	}

	terminalLeft := float64(w) - 0.15*state.scale
	terminalRight := float64(aw) - math.Max(0.4, 0.35*state.scale)
	terminalTop := float64(h) * 0.30
	terminalBottom := float64(h) * 0.70

	outlineColor := [3]float64{245, 245, 248}
	fillColor := [3]float64{33, 147, 255}

	for py := 0; py < int(h); py++ {
		for px := 0; px < int(aw); px++ {
			var sumR, sumG, sumB float64
			covered := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/samples
					fy := float64(py) + (float64(sy)+0.5)/samples

					isOutline := false
					if fx < float64(w) {
						sd := roundedBoxSDF(fx, fy, bodyCX, bodyCY, bodyHalfW, bodyHalfH, bodyRadius)
						isOutline = math.Abs(sd) <= stroke/2
					}
					isTerminal := fx >= terminalLeft && fx <= terminalRight && fy >= terminalTop && fy <= terminalBottom
					isFill := known && fillRight > innerLeft && fx >= innerLeft && fx <= fillRight && fy >= innerTop && fy <= innerBottom

					if isOutline || isTerminal {
						sumR += outlineColor[0]
						sumG += outlineColor[1]
						sumB += outlineColor[2]
						covered++
					} else if isFill {
						sumR += fillColor[0]
						sumG += fillColor[1]
						sumB += fillColor[2]
						covered++
					}
				}
			}
			if covered == 0 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = byte(math.Round(sumR / float64(covered)))
			canvas.Pix[i+1] = byte(math.Round(sumG / float64(covered)))
			canvas.Pix[i+2] = byte(math.Round(sumB / float64(covered)))
			canvas.Pix[i+3] = byte(math.Round(255 * float64(covered) / float64(totalSamples)))
		}
	}

	artwork, ok := artworkFromNRGBA(canvas)
	if ok {
		selectionFrameCache[key] = artwork
	}
	return artwork, ok
}

func drawBatteryIcon(hdc uintptr, x, y int32, pct int, known bool) {
	w := scaled(30)
	h := scaled(14)
	artwork, ok := batteryIconArtwork(pct, known, w, h)
	if !ok {
		// Fallback simples caso a criação do bitmap não esteja disponível.
		roundedOutline(hdc, rect{x, y, x + w, y + h}, rgb(245, 245, 248), scaled(6), scaledStroke(1))
		fillRect(hdc, rect{x + w, y + scaled(4), x + w + scaled(3), y + h - scaled(4)}, rgb(245, 245, 248))
		return
	}
	drawArtworkBitmap(hdc, rect{x, y, x + artwork.w, y + artwork.h}, artwork)
}

func drawBatteryGlassCard(hdc uintptr, c rect) {
	w := c.Right - c.Left
	h := c.Bottom - c.Top
	if w <= 0 || h <= 0 {
		return
	}
	key := fmt.Sprintf("battery-mockup-card-polished:%d:%d:%d", w, h, int(math.Round(state.scale*1000)))
	artwork, ok := selectionFrameCache[key]
	if !ok {
		artwork, ok = makeBatteryGlassCardArtwork(w, h)
		if ok {
			selectionFrameCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, rect{0, 0, w, h}, artwork)
		return
	}
	// Fallback: mantém o overlay utilizável caso a criação da arte falhe.
	rounded(hdc, rect{0, 0, w, h}, rgb(5, 7, 12), rgb(82, 88, 99), scaled(12), scaledStroke(1))
}

const (
	overlayGlassBaseRed   = 5.0
	overlayGlassBaseGreen = 7.0
	overlayGlassBaseBlue  = 12.0
)

// drawFullscreenWarningCard mantém o aviso legível sobre qualquer jogo e usa
// a mesma paleta de fundo da cápsula compacta da bateria.
func drawFullscreenWarningCard(hdc uintptr, c rect) {
	w := c.Right - c.Left
	h := c.Bottom - c.Top
	if w <= 0 || h <= 0 {
		return
	}
	key := fmt.Sprintf("fullscreen-warning-card-v4-battery-glass:%d:%d:%d", w, h, int(math.Round(state.scale*1000)))
	artwork, ok := selectionFrameCache[key]
	if !ok {
		artwork, ok = makeFullscreenWarningCardArtwork(w, h)
		if ok {
			selectionFrameCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, rect{0, 0, w, h}, artwork)
		return
	}

	// Fallback simples para máquinas em que a superfície antialiasada falhe.
	padding := scaled(fullscreenWarningShadowPadding)
	rounded(hdc, rect{padding, padding, w - padding, h - padding}, rgb(5, 7, 12), rgb(125, 132, 146), scaled(18), scaledStroke(1))
}

func makeFullscreenWarningCardArtwork(w, h int32) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	scale := math.Max(0.50, state.scale)
	// O cartão fica afastado do limite da DIB. Como o halo termina antes dessa
	// margem, inclusive os cantos arredondados chegam a alpha zero sem corte.
	margin := float64(fullscreenWarningShadowPadding) * scale
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	halfW := float64(w-1)/2 - margin
	halfH := float64(h-1)/2 - margin
	radius := math.Min(18.0*scale, math.Min(halfW, halfH))
	if radius < 6 {
		radius = 6
	}

	const samples = 4
	const totalSamples = samples * samples
	for py := 0; py < int(h); py++ {
		for px := 0; px < int(w); px++ {
			var sumPremulR, sumPremulG, sumPremulB, sumA float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/samples
					fy := float64(py) + (float64(sy)+0.5)/samples
					sd := roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, radius)

					var a, pr, pg, pb float64
					compose := func(r, g, b, sourceAlpha float64) {
						if sourceAlpha <= 0 {
							return
						}
						if sourceAlpha > 1 {
							sourceAlpha = 1
						}
						inv := 1 - sourceAlpha
						pr = r*sourceAlpha + pr*inv
						pg = g*sourceAlpha + pg*inv
						pb = b*sourceAlpha + pb*inv
						a = sourceAlpha + a*inv
					}

					outside := math.Max(sd, 0)
					if outside <= fullscreenWarningShadowExtent*scale {
						shadow := math.Exp(-(outside * outside) / (2 * math.Pow(4.2*scale, 2)))
						compose(0, 0, 0, shadow*0.34)
					}

					if sd <= 0 {
						// Mesma base grafite-azulada do overlay compacto de bateria. O aviso
						// usa mais opacidade apenas para manter o texto legível sobre o jogo.
						compose(overlayGlassBaseRed, overlayGlassBaseGreen, overlayGlassBaseBlue, 0.90)

						// Reflexo discreto à esquerda preserva a sensação de vidro.
						leftDistance := math.Max(0, fx-margin)
						leftGlow := math.Exp(-(leftDistance * leftDistance) / (2 * math.Pow(125*scale, 2)))
						verticalFocus := math.Exp(-math.Pow((fy-cy)/(0.72*float64(h)), 2) / 2)
						compose(80, 88, 104, leftGlow*verticalFocus*0.028)

						// Reflexo fino no topo e vinheta inferior dão profundidade sem ruído.
						topGlow := math.Exp(-math.Pow((fy-margin)/(9.0*scale), 2) / 2)
						compose(235, 235, 235, topGlow*0.050)
						bottomShade := math.Max(0, (fy-cy)/math.Max(1, halfH))
						compose(0, 0, 0, bottomShade*0.10)
					}

					borderDistance := math.Abs(sd)
					if borderDistance <= 1.30*scale {
						core := math.Exp(-(borderDistance * borderDistance) / (2 * math.Pow(0.58*scale, 2)))
						compose(125, 132, 146, core*0.48)
					}

					if a > 0.0001 {
						sumPremulR += pr
						sumPremulG += pg
						sumPremulB += pb
						sumA += a
					}
				}
			}

			avgA := sumA / totalSamples
			if avgA <= 0.002 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = byte(math.Round(math.Min(255, sumPremulR/sumA)))
			canvas.Pix[i+1] = byte(math.Round(math.Min(255, sumPremulG/sumA)))
			canvas.Pix[i+2] = byte(math.Round(math.Min(255, sumPremulB/sumA)))
			canvas.Pix[i+3] = byte(math.Round(255 * math.Min(1, avgA)))
		}
	}
	return artworkFromNRGBA(canvas)
}

func makeBatteryGlassCardArtwork(w, h int32) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	scale := state.scale
	if scale < 0.75 {
		scale = 0.75
	}
	margin := 1.15 * scale
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	halfW := float64(w-1)/2 - margin
	halfH := float64(h-1)/2 - margin
	radius := 12.0 * scale
	maxRadius := math.Min(halfW, halfH)
	if radius > maxRadius {
		radius = maxRadius
	}
	if radius < 5 {
		radius = 5
	}
	const samples = 4
	const totalSamples = samples * samples
	for py := 0; py < int(h); py++ {
		for px := 0; px < int(w); px++ {
			var sumPremulR, sumPremulG, sumPremulB, sumA float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/samples
					fy := float64(py) + (float64(sy)+0.5)/samples
					sd := roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, radius)
					var a, pr, pg, pb float64
					compose := func(r, g, b, sa float64) {
						if sa <= 0 {
							return
						}
						if sa > 1 {
							sa = 1
						}
						inv := 1 - sa
						pr = r*sa + pr*inv
						pg = g*sa + pg*inv
						pb = b*sa + pb*inv
						a = sa + a*inv
					}

					outside := math.Max(sd, 0)
					if outside < 5.0*scale {
						shadow := math.Exp(-(outside * outside) / (2 * math.Pow(2.6*scale, 2)))
						compose(0, 0, 0, shadow*0.20)
					}

					if sd <= 0 {
						// Fundo preto translúcido, igual à cápsula do mockup.
						compose(overlayGlassBaseRed, overlayGlassBaseGreen, overlayGlassBaseBlue, 0.58)
						// Reflexo branco muito leve no topo para parecer vidro, sem borda azul.
						topGlow := math.Exp(-math.Pow((fy-margin)/(8.0*scale), 2) / 2)
						compose(255, 255, 255, topGlow*0.045)
						// Miolo ligeiramente mais claro à esquerda, como no preview aprovado.
						leftGlow := math.Exp(-math.Pow((fx-margin)/(18.0*scale), 2) / 2)
						compose(80, 88, 104, leftGlow*0.028)
					}

					borderDistance := math.Abs(sd)
					if borderDistance <= 1.05*scale {
						core := math.Exp(-(borderDistance * borderDistance) / (2 * math.Pow(0.50*scale, 2)))
						compose(125, 132, 146, core*0.48)
					}

					if a > 0.0001 {
						sumPremulR += pr
						sumPremulG += pg
						sumPremulB += pb
						sumA += a
					}
				}
			}
			avgA := sumA / totalSamples
			if avgA <= 0.002 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = byte(math.Round(math.Min(255, sumPremulR/sumA)))
			canvas.Pix[i+1] = byte(math.Round(math.Min(255, sumPremulG/sumA)))
			canvas.Pix[i+2] = byte(math.Round(math.Min(255, sumPremulB/sumA)))
			canvas.Pix[i+3] = byte(math.Round(255 * math.Min(1, avgA)))
		}
	}
	return artworkFromNRGBA(canvas)
}

var batteryOverlayAccent = rgb(33, 147, 255)

func batteryIconArtworkAccent(pct int, known bool, w, h int32, fill uintptr) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	pct = clamp(pct, 0, 100)
	key := fmt.Sprintf("battery-accent-polished:%d:%d:%d:%t:%d:%d", w, h, pct, known, fill, int(math.Round(state.scale*1000)))
	if artwork, ok := selectionFrameCache[key]; ok {
		return artwork, true
	}

	scale := state.scale
	if scale < 0.75 {
		scale = 0.75
	}
	capW := int32(math.Round(4.4 * scale))
	if capW < 3 {
		capW = 3
	}
	aw := w + capW
	canvas := image.NewNRGBA(image.Rect(0, 0, int(aw), int(h)))

	const samples = 4
	const totalSamples = samples * samples
	bodyInset := math.Max(0.80, 0.70*scale)
	bodyCX := bodyInset + (float64(w)-2*bodyInset)/2
	bodyCY := bodyInset + (float64(h)-2*bodyInset)/2
	bodyHalfW := (float64(w) - 2*bodyInset) / 2
	bodyHalfH := (float64(h) - 2*bodyInset) / 2
	bodyRadius := math.Min(3.2*scale, bodyHalfH)
	stroke := math.Max(1.20, 1.38*scale)

	innerLeft := bodyInset + stroke + 2.0*scale
	innerRight := float64(w) - bodyInset - stroke - 2.0*scale
	innerTop := bodyInset + stroke + 2.0*scale
	innerBottom := float64(h) - bodyInset - stroke - 2.0*scale
	if innerRight <= innerLeft || innerBottom <= innerTop {
		return controllerArtwork{}, false
	}
	innerW := innerRight - innerLeft
	innerH := innerBottom - innerTop
	fillRight := innerLeft
	if known {
		fillRight = innerLeft + innerW*float64(pct)/100.0
	}
	// O recorte inclinado evita preenchimento reto/genérico e mantém o miolo
	// azul com uma pequena sobra escura no lado direito.
	slant := math.Min(5.2*scale, innerW*0.22)

	terminalLeft := float64(w) - 0.05*scale
	terminalRight := float64(aw) - math.Max(0.55, 0.45*scale)
	terminalTop := float64(h) * 0.34
	terminalBottom := float64(h) * 0.66
	terminalRadius := math.Min(1.6*scale, (terminalBottom-terminalTop)/2)

	fillR := float64(byte(fill))
	fillG := float64(byte(fill >> 8))
	fillB := float64(byte(fill >> 16))
	white := [3]float64{248, 248, 250}

	for py := 0; py < int(h); py++ {
		for px := 0; px < int(aw); px++ {
			var sumPremulR, sumPremulG, sumPremulB, sumA float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/samples
					fy := float64(py) + (float64(sy)+0.5)/samples
					var a, pr, pg, pb float64
					compose := func(r, g, b, sa float64) {
						if sa <= 0 {
							return
						}
						if sa > 1 {
							sa = 1
						}
						inv := 1 - sa
						pr = r*sa + pr*inv
						pg = g*sa + pg*inv
						pb = b*sa + pb*inv
						a = sa + a*inv
					}

					sd := 999.0
					if fx < float64(w) {
						sd = roundedBoxSDF(fx, fy, bodyCX, bodyCY, bodyHalfW, bodyHalfH, bodyRadius)
					}

					if sd <= -stroke/2 {
						// Fundo interno escuro antes do preenchimento azul.
						compose(1, 4, 8, 0.96)
						insideInner := fx >= innerLeft && fx <= innerRight && fy >= innerTop && fy <= innerBottom
						if insideInner && known {
							t := (fy - innerTop) / innerH
							capacityEdge := fillRight - slant*(0.50-t)
							if fx <= capacityEdge {
								xn := (fx - innerLeft) / innerW
								yn := (fy - innerTop) / innerH
								brightness := 1.0 + 0.16*(1.0-yn) - 0.20*xn
								compose(fillR*brightness, fillG*brightness, fillB*brightness, 0.98)
								topGlow := math.Exp(-math.Pow(yn/0.24, 2) / 2)
								compose(150, 220, 255, topGlow*0.20)
								slashCenter := innerLeft + innerW*0.62 + (fy-innerTop-innerH*0.5)*0.48
								slashDist := math.Abs(fx - slashCenter)
								if slashDist <= 2.1*scale && xn > 0.45 {
									shade := 1 - slashDist/(2.1*scale)
									compose(2, 10, 24, shade*0.42)
								}
								if xn > 0.72 {
									compose(2, 8, 20, (xn-0.72)/0.28*0.22)
								}
							}
						}
					}

					borderDistance := math.Abs(sd)
					if borderDistance <= stroke/2 {
						core := math.Exp(-(borderDistance * borderDistance) / (2 * math.Pow(0.36*scale, 2)))
						compose(white[0], white[1], white[2], 0.92*core)
					}
					if fx >= terminalLeft && fx <= terminalRight && fy >= terminalTop && fy <= terminalBottom {
						tcx := (terminalLeft + terminalRight) / 2
						tcy := (terminalTop + terminalBottom) / 2
						tsd := roundedBoxSDF(fx, fy, tcx, tcy, (terminalRight-terminalLeft)/2, (terminalBottom-terminalTop)/2, terminalRadius)
						if tsd <= 0 {
							compose(white[0], white[1], white[2], 0.90)
						}
					}

					if a > 0.0001 {
						sumPremulR += pr * a
						sumPremulG += pg * a
						sumPremulB += pb * a
						sumA += a
					}
				}
			}
			avgA := sumA / totalSamples
			if avgA <= 0.002 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = byte(math.Round(math.Min(255, sumPremulR/sumA)))
			canvas.Pix[i+1] = byte(math.Round(math.Min(255, sumPremulG/sumA)))
			canvas.Pix[i+2] = byte(math.Round(math.Min(255, sumPremulB/sumA)))
			canvas.Pix[i+3] = byte(math.Round(255 * math.Min(1, avgA)))
		}
	}

	artwork, ok := artworkFromNRGBA(canvas)
	if ok {
		selectionFrameCache[key] = artwork
	}
	return artwork, ok
}

func paintBattery(hdc uintptr, c rect) {
	state.mu.Lock()
	known := state.batteryKnown
	pct := state.batteryPercent
	state.mu.Unlock()

	drawBatteryGlassCard(hdc, c)

	// Controle dedicado apenas para o overlay de bateria.
	drawBatteryController(hdc, rect{scaled(12), 0, scaled(72), c.Bottom})

	batteryW := scaled(35)
	batteryH := scaled(18)
	batteryX := scaled(76)
	batteryY := (c.Bottom - batteryH) / 2
	if artwork, ok := batteryIconArtworkAccent(pct, known, batteryW, batteryH, batteryOverlayAccent); ok {
		drawArtworkBitmap(hdc, rect{batteryX, batteryY, batteryX + artwork.w, batteryY + artwork.h}, artwork)
	} else {
		drawBatteryIcon(hdc, batteryX, batteryY, pct, known)
	}

	pctText := "--%"
	if known {
		pctText = fmt.Sprintf("%d%%", clamp(pct, 0, 100))
	}
	textFont(hdc, pctText, rect{scaled(126), 0, c.Right - scaled(12), c.Bottom}, rgb(252, 252, 253), scaled(20), FW_NORMAL, "Segoe UI Variable Text", DT_LEFT|DT_VCENTER|DT_SINGLELINE)
}

const fullscreenWarningTextFlags = DT_CENTER | DT_VCENTER | DT_SINGLELINE

func fullscreenWarningTextRects(c rect) (rect, rect) {
	padding := scaled(fullscreenWarningShadowPadding)
	cardLeft := padding
	cardRight := c.Right - padding
	cardBottom := c.Bottom - padding
	title := rect{cardLeft + scaled(24), padding + scaled(14), cardRight - scaled(24), padding + scaled(46)}
	body := rect{cardLeft + scaled(24), padding + scaled(48), cardRight - scaled(24), cardBottom - scaled(14)}
	return title, body
}

func paintMessage(hdc uintptr, c rect) {
	state.mu.Lock()
	title := state.messageTitle
	body := state.messageBody
	state.mu.Unlock()

	if strings.EqualFold(title, fullscreenUnavailableTitle) {
		drawFullscreenWarningCard(hdc, c)
		titleRect, bodyRect := fullscreenWarningTextRects(c)

		textFont(
			hdc,
			title,
			titleRect,
			rgb(250, 250, 250),
			scaled(21),
			FW_SEMIBOLD,
			"Segoe UI Variable Display",
			fullscreenWarningTextFlags,
		)
		textFont(
			hdc,
			body,
			bodyRect,
			rgb(196, 196, 196),
			scaled(15),
			FW_MEDIUM,
			"Segoe UI Variable Text",
			fullscreenWarningTextFlags,
		)
		return
	}

	card := rect{scaled(5), scaled(5), c.Right - scaled(5), c.Bottom - scaled(5)}
	drawPS5Card(hdc, card)

	isGameMessage := strings.EqualFold(title, "MODO JOGO")
	if isGameMessage {
		iconColor(hdc, "\uE7E8", rect{scaled(32), scaled(30), scaled(128), c.Bottom - scaled(30)}, scaled(56), rgb(245, 245, 248))
		line(hdc, scaled(56), scaled(118), scaled(108), scaled(48), rgb(42, 156, 255))
	} else {
		iconColor(hdc, "\uE946", rect{scaled(32), scaled(30), scaled(128), c.Bottom - scaled(30)}, scaled(54), rgb(42, 156, 255))
	}
	line(hdc, scaled(152), scaled(28), scaled(152), c.Bottom-scaled(28), rgb(70, 75, 84))

	textFont(hdc, title, rect{scaled(182), scaled(18), c.Right - scaled(24), scaled(53)}, rgb(42, 156, 255), scaled(15), FW_SEMIBOLD, "Segoe UI Variable Display", DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	bodySize := int32(18)
	if isGameMessage {
		bodySize = 24
	}
	text(hdc, body, rect{scaled(182), scaled(48), c.Right - scaled(24), scaled(104)}, rgb(245, 245, 248), scaled(bodySize), FW_SEMIBOLD, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}
