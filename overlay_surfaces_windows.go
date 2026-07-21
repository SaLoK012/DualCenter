// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"fmt"
	"image"
	"math"
)

// supersamplingForScale mantém a suavidade física das curvas sem multiplicar
// desnecessariamente o trabalho em telas de alta densidade. Em 4K cada pixel já
// representa metade do tamanho visual de um pixel em 1080p.
func supersamplingForScale(reference int) int {
	scale := math.Max(1, state.scale)
	samples := int(math.Round(float64(reference) / scale))
	return clamp(samples, 1, reference)
}

// menuDockSurface cria somente uma sombra preta suave atrás do conjunto de
// cartões. As abas continuam responsáveis pelo próprio corpo, contorno e brilho.
func menuDockSurface(hdc uintptr, c rect) {
	w := c.Right - c.Left
	h := c.Bottom - c.Top
	if w <= 0 || h <= 0 {
		return
	}
	key := fmt.Sprintf("menu-shadow-v6:%d:%d:%d", w, h, int(math.Round(state.scale*1000)))
	artwork, ok := panelSurfaceCache[key]
	if !ok {
		artwork, ok = makeMenuDockArtwork(w, h)
		if ok {
			panelSurfaceCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, rect{0, 0, w, h}, artwork)
	}
}

func makeMenuDockArtwork(w, h int32) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	scale := math.Max(0.75, state.scale)
	// A silhueta acompanha exatamente o conjunto das cinco abas. A janela tem
	// uma margem transparente própria, permitindo que a sombra cresça para fora.
	first := panelRect(0)
	last := panelRect(4)
	cx := float64(first.Left+last.Right) / 2
	cy := float64(first.Top+first.Bottom) / 2
	halfW := float64(last.Right-first.Left) / 2
	halfH := float64(first.Bottom-first.Top) / 2
	if halfW <= 0 || halfH <= 0 {
		return controllerArtwork{}, false
	}
	radius := math.Min(30*scale, math.Min(halfW, halfH))
	sigma := 14.5 * scale
	shadowExtent := 30.0 * scale
	const horizontalStretch = 1.15
	const verticalStretch = 1.65

	// A amostragem de até 4x4 elimina degraus nas curvas. O pequeno dithering de
	// alpha quebra faixas de quantização sem introduzir granulação perceptível.
	samples := supersamplingForScale(4)
	totalSamples := samples * samples
	bayer4 := [...]float64{
		0, 8, 2, 10,
		12, 4, 14, 6,
		3, 11, 1, 9,
		15, 7, 13, 5,
	}
	for py := 0; py < int(h); py++ {
		for px := 0; px < int(w); px++ {
			var sumA float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/float64(samples)
					fy := float64(py) + (float64(sy)+0.5)/float64(samples)
					// Comprima as coordenadas usadas na distância. A borda continua presa
					// às abas, enquanto a dissipação cresce mais na vertical e um pouco nas laterais.
					shadowX := cx + (fx-cx)/horizontalStretch
					shadowY := cy + (fy-cy)/verticalStretch
					sd := roundedBoxSDF(shadowX, shadowY, cx, cy, halfW/horizontalStretch, halfH/verticalStretch, radius)
					outside := math.Max(sd, 0)
					if outside >= shadowExtent {
						continue
					}
					t := outside / shadowExtent
					// O smoothstep chega a zero com derivada zero; não existe um corte
					// de alpha capaz de formar uma linha no fim do degradê.
					taper := 1 - t*t*(3-2*t)
					fade := math.Exp(-(outside*outside)/(2*sigma*sigma)) * taper
					// O preto ganha mais presença atrás das abas e se dissolve nas
					// bordas. O peso discretamente maior abaixo reforça a profundidade.
					vertical := 0.84 + 0.16*math.Max(0, (fy-cy)/math.Max(1, halfH))
					sumA += 0.70 * fade * vertical
				}
			}
			avgA := sumA / float64(totalSamples)
			if avgA <= 0 {
				continue
			}
			alpha := 255*math.Min(1, avgA) + bayer4[(py&3)*4+(px&3)]/16 - 0.46875
			if alpha <= 0 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = 0
			canvas.Pix[i+1] = 0
			canvas.Pix[i+2] = 0
			canvas.Pix[i+3] = byte(math.Round(math.Min(255, alpha)))
		}
	}
	return artworkFromNRGBA(canvas)
}

// panelSurface desenha o corpo grafite da aba e aplica o contorno em uma
// segunda camada antialiasada. Assim, a mesma geometria é usada no estado
// normal e selecionado, enquanto o neon selecionado pode crescer para fora da
// aba exatamente como nos retângulos internos.
func panelSurface(hdc uintptr, r rect, selected bool) {
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w <= 0 || h <= 0 {
		return
	}
	radius := panelCornerRadius()

	// No modo layered, corpo, contorno e brilho são uma única composição com
	// alpha real. A geometria é idêntica em todas as camadas, eliminando a
	// linha preta depois da borda cinza e o retângulo preto ao redor do efeito.
	if activeLayeredFrame != nil {
		margin := scaled(3)
		if selected {
			margin = scaled(10)
		}
		if margin < 2 {
			margin = 2
		}
		animationFrame := selectionTransitionFrames
		if selected {
			animationFrame = state.selectionFrame
		}
		key := fmt.Sprintf("panel-composite-v2:%t:%d:%d:%d:%d:%d:%d", selected, w, h, radius, margin, animationFrame, int(math.Round(state.scale*1000)))
		artwork, ok := panelSurfaceCache[key]
		if !ok {
			artwork, ok = makePanelCompositeArtwork(w, h, radius, margin, selected)
			if ok {
				panelSurfaceCache[key] = artwork
			}
		}
		if ok {
			drawArtworkBitmap(hdc, rect{r.Left - margin, r.Top - margin, r.Right + margin, r.Bottom + margin}, artwork)
			return
		}
	}

	// Fallback legado caso a criação do frame layered falhe.
	key := fmt.Sprintf("panel-body:%d:%d:%d:%d", w, h, radius, int(math.Round(state.scale*1000)))
	artwork, ok := panelSurfaceCache[key]
	if !ok {
		artwork, ok = makePanelSurfaceArtwork(w, h, radius)
		if ok {
			panelSurfaceCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, r, artwork)
	} else {
		rounded(hdc, r, rgb(12, 15, 20), rgb(12, 15, 20), radius*2, scaledStroke(1))
	}
	if selected {
		panelSelectionNeon(hdc, r, radius)
	} else {
		panelNeutralOutline(hdc, r, radius)
	}
}

func makePanelCompositeArtwork(w, h, radius, margin int32, selected bool) (controllerArtwork, bool) {
	aw := w + margin*2
	ah := h + margin*2
	if aw <= 0 || ah <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(aw), int(ah)))
	cx := float64(margin) + float64(w-1)/2
	cy := float64(margin) + float64(h-1)/2
	halfW := float64(w-1) / 2
	halfH := float64(h-1) / 2
	rr := math.Min(float64(radius), math.Min(halfW, halfH))
	if rr < 1 {
		rr = 1
	}
	scale := math.Max(0.75, state.scale)
	sigma := 2.35 * scale
	coreSigma := math.Max(0.55, 0.62*scale)
	neutralSigma := math.Max(0.48, 0.52*scale)
	selectionStrength := selectionTransitionStrengthLocked()

	// A arte é gerada uma única vez por resolução e reutilizada. Até 8x8 deixa os
	// cantos suaves; em alta densidade a amostragem diminui sem perda física.
	samples := supersamplingForScale(8)
	totalSamples := samples * samples
	for py := 0; py < int(ah); py++ {
		for px := 0; px < int(aw); px++ {
			var sumA, sumPR, sumPG, sumPB float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/float64(samples)
					fy := float64(py) + (float64(sy)+0.5)/float64(samples)
					sd := roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, rr)

					// Composição source-over em variáveis pré-multiplicadas.
					a, pr, pg, pb := 0.0, 0.0, 0.0, 0.0
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

					if selected && sd >= 0 {
						glow := math.Exp(-(sd * sd) / (2 * sigma * sigma))
						if glow > 0.008 {
							compose(18, 112, 238, glow*72.0/255.0*selectionStrength)
						}
					}

					if sd <= 0 {
						compose(12, 15, 20, 0.97)
						if selected {
							compose(0, 92, 178, 0.065*selectionStrength)
						}
					}

					borderDistance := math.Abs(sd)
					if selected {
						glow := 0.0
						if sd >= -0.45*scale {
							outsideDistance := math.Max(sd, 0)
							glow = math.Exp(-(outsideDistance * outsideDistance) / (2 * sigma * sigma))
						}
						core := math.Exp(-(borderDistance * borderDistance) / (2 * coreSigma * coreSigma))
						if sd < -1.35*scale {
							core = 0
						}
						borderA := math.Min(1, math.Max(glow*72, core*230)/255) * selectionStrength
						mix := math.Min(1, core)
						compose(24+105*mix, 108+92*mix, 232+20*mix, borderA)
					} else {
						core := math.Exp(-(borderDistance * borderDistance) / (2 * neutralSigma * neutralSigma))
						// A borda neutra fica para dentro da mesma silhueta. Não sobra
						// nenhum pixel preto depois da linha cinza.
						if sd > 0.30*scale || sd < -1.30*scale {
							core = 0
						}
						compose(55, 61, 70, core*0.88)
					}

					sumA += a
					sumPR += pr
					sumPG += pg
					sumPB += pb
				}
			}
			avgA := sumA / float64(totalSamples)
			if avgA <= 0.001 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = byte(math.Round(math.Min(255, (sumPR / sumA))))
			canvas.Pix[i+1] = byte(math.Round(math.Min(255, (sumPG / sumA))))
			canvas.Pix[i+2] = byte(math.Round(math.Min(255, (sumPB / sumA))))
			canvas.Pix[i+3] = byte(math.Round(255 * math.Min(1, avgA)))
		}
	}
	return artworkFromNRGBA(canvas)
}

// makePanelSurfaceArtwork rasteriza apenas o corpo preto. A amostragem 8x8
// melhora a decisão dos pixels da silhueta nos cantos e o contorno separado
// cobre a transição com antialiasing real, evitando degraus visíveis.
func makePanelSurfaceArtwork(w, h, radius int32) (controllerArtwork, bool) {
	if w <= 0 || h <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	halfW := float64(w-1) / 2
	halfH := float64(h-1) / 2
	rr := float64(radius)
	maxRadius := math.Min(halfW, halfH)
	if rr > maxRadius {
		rr = maxRadius
	}
	if rr < 1 {
		rr = 1
	}

	samples := supersamplingForScale(8)
	totalSamples := samples * samples
	for py := 0; py < int(h); py++ {
		for px := 0; px < int(w); px++ {
			insideSamples := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/float64(samples)
					fy := float64(py) + (float64(sy)+0.5)/float64(samples)
					if roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, rr) <= 0 {
						insideSamples++
					}
				}
			}
			// A janela usa color key; por isso, o corpo precisa continuar opaco.
			// O contorno antialiasado separado suaviza visualmente esta borda.
			if insideSamples*2 < totalSamples {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = 0
			canvas.Pix[i+1] = 0
			canvas.Pix[i+2] = 0
			canvas.Pix[i+3] = 255
		}
	}
	return artworkFromNRGBA(canvas)
}

// selectionNeon mantém o preenchimento azul discreto somente nos retângulos
// internos. A linha e o halo são compartilhados com a seleção das abas.
func selectionNeon(hdc uintptr, r rect, radius int32) {
	drawNeonFrame(hdc, r, radius, true)
}

// panelSelectionNeon usa exatamente a linha, a cor e o brilho externo dos
// retângulos selecionadores, mas sem qualquer preenchimento azul dentro da aba.
func panelSelectionNeon(hdc uintptr, r rect, radius int32) {
	drawNeonFrame(hdc, r, radius, false)
}

func drawNeonFrame(hdc uintptr, r rect, radius int32, withFill bool) {
	drawNeonFrameMode(hdc, r, radius, withFill, false)
}

// compactSelectedOption compõe o fundo preto e a seleção neon na mesma
// geometria SDF. O mini menu fica com uma única moldura, sem a caixa cinza
// externa e sem o segundo retângulo encaixado que deixavam a opção pesada.
func compactSelectedOption(hdc uintptr, r rect, radius int32) {
	drawNeonFrameMode(hdc, r, radius, true, true)
}

func drawNeonFrameMode(hdc uintptr, r rect, radius int32, withFill, opaqueBase bool) {
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w <= 0 || h <= 0 {
		return
	}
	margin := scaled(8)
	if margin < 6 {
		margin = 6
	}
	key := fmt.Sprintf("neon-v2:%t:%t:%d:%d:%d:%d:%d:%d", withFill, opaqueBase, w, h, radius, margin, state.selectionFrame, int(math.Round(state.scale*1000)))
	artwork, ok := selectionFrameCache[key]
	if !ok {
		artwork, ok = makeSelectionFrameArtwork(w, h, radius, margin, withFill, opaqueBase)
		if ok {
			selectionFrameCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, rect{r.Left - margin, r.Top - margin, r.Right + margin, r.Bottom + margin}, artwork)
		return
	}

	// Fallback para ambientes em que a criação do bitmap falhe.
	if opaqueBase {
		rounded(hdc, r, rgb(12, 15, 20), rgb(12, 15, 20), radius*2, scaledStroke(1))
	}
	roundedOutline(hdc, r, rgb(42, 148, 238), radius*2, scaledStroke(1))
}

// panelNeutralOutline desenha a borda cinza normal em uma camada SDF
// antialiasada. Dessa forma, todos os cantos permanecem igualmente suaves,
// mesmo quando a aba não está selecionada.
func panelNeutralOutline(hdc uintptr, r rect, radius int32) {
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w <= 0 || h <= 0 {
		return
	}
	margin := scaled(3)
	if margin < 2 {
		margin = 2
	}
	key := fmt.Sprintf("panel-outline:%d:%d:%d:%d:%d", w, h, radius, margin, int(math.Round(state.scale*1000)))
	artwork, ok := selectionFrameCache[key]
	if !ok {
		artwork, ok = makePanelNeutralOutlineArtwork(w, h, radius, margin)
		if ok {
			selectionFrameCache[key] = artwork
		}
	}
	if ok {
		drawArtworkBitmap(hdc, rect{r.Left - margin, r.Top - margin, r.Right + margin, r.Bottom + margin}, artwork)
		return
	}
	roundedOutline(hdc, r, rgb(48, 52, 59), radius*2, scaledStroke(1))
}

func roundedBoxSDF(x, y, cx, cy, halfW, halfH, radius float64) float64 {
	qx := math.Abs(x-cx) - (halfW - radius)
	qy := math.Abs(y-cy) - (halfH - radius)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	outside := math.Hypot(ox, oy)
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - radius
}

func makeSelectionFrameArtwork(w, h, radius, margin int32, withFill, opaqueBase bool) (controllerArtwork, bool) {
	aw := w + margin*2
	ah := h + margin*2
	if aw <= 0 || ah <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(aw), int(ah)))
	cx := float64(margin) + float64(w-1)/2
	cy := float64(margin) + float64(h-1)/2
	halfW := float64(w-1) / 2
	halfH := float64(h-1) / 2
	rr := float64(radius)
	maxRadius := math.Min(halfW, halfH)
	if rr > maxRadius {
		rr = maxRadius
	}
	if rr < 1 {
		rr = 1
	}
	scale := state.scale
	if scale < 0.75 {
		scale = 0.75
	}
	sigma := 2.35 * scale
	coreSigma := 0.62 * scale
	if coreSigma < 0.55 {
		coreSigma = 0.55
	}
	selectionStrength := selectionTransitionStrengthLocked()

	// A mesma moldura é usada por abas, opções, mosaicos e menu contextual.
	// A amostragem de até 4x4 suaviza especialmente as curvas grandes sem alterar
	// a cor, a espessura ou a intensidade aprovadas do efeito.
	samples := supersamplingForScale(4)
	totalSamples := samples * samples
	for py := 0; py < int(ah); py++ {
		for px := 0; px < int(aw); px++ {
			var sumPremulR, sumPremulG, sumPremulB, sumA float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/float64(samples)
					fy := float64(py) + (float64(sy)+0.5)/float64(samples)
					signedDistance := roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, rr)
					borderDistance := math.Abs(signedDistance)

					glow := 0.0
					if signedDistance >= -0.45*scale {
						outsideDistance := math.Max(signedDistance, 0)
						glow = math.Exp(-(outsideDistance * outsideDistance) / (2 * sigma * sigma))
					}
					core := math.Exp(-(borderDistance * borderDistance) / (2 * coreSigma * coreSigma))
					if signedDistance < -1.15*scale {
						core = 0
					}
					borderA := math.Min(1, math.Max(glow*72, core*230)/255) * selectionStrength
					fillA := 0.0
					if withFill && signedDistance < 0 {
						fillA = 0.075 * selectionStrength
						if signedDistance > -0.8*scale {
							fillA *= math.Min(1, -signedDistance/(0.8*scale))
						}
					}
					baseA := 0.0
					if opaqueBase && signedDistance < 0 {
						baseA = 1
					}
					underA := fillA + baseA*(1-fillA)
					outA := borderA + underA*(1-borderA)
					if outA <= 0.0001 {
						continue
					}

					mix := math.Min(1, core)
					borderR := 24.0 + 105.0*mix
					borderG := 108.0 + 92.0*mix
					borderB := 232.0 + 20.0*mix
					fillR := 0.0
					fillG := 92.0
					fillB := 178.0
					baseVisible := baseA * (1 - fillA) * (1 - borderA)
					// O mini menu usa a mesma base grafite dos cartões principais.
					sumPremulR += borderR*borderA + fillR*fillA*(1-borderA) + 12*baseVisible
					sumPremulG += borderG*borderA + fillG*fillA*(1-borderA) + 15*baseVisible
					sumPremulB += borderB*borderA + fillB*fillA*(1-borderA) + 20*baseVisible
					sumA += outA
				}
			}
			avgA := sumA / float64(totalSamples)
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

func makePanelNeutralOutlineArtwork(w, h, radius, margin int32) (controllerArtwork, bool) {
	aw := w + margin*2
	ah := h + margin*2
	if aw <= 0 || ah <= 0 {
		return controllerArtwork{}, false
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, int(aw), int(ah)))
	cx := float64(margin) + float64(w-1)/2
	cy := float64(margin) + float64(h-1)/2
	halfW := float64(w-1) / 2
	halfH := float64(h-1) / 2
	rr := float64(radius)
	maxRadius := math.Min(halfW, halfH)
	if rr > maxRadius {
		rr = maxRadius
	}
	if rr < 1 {
		rr = 1
	}
	scale := state.scale
	if scale < 0.75 {
		scale = 0.75
	}
	coreSigma := math.Max(0.48, 0.52*scale)
	samples := supersamplingForScale(4)
	totalSamples := samples * samples
	for py := 0; py < int(ah); py++ {
		for px := 0; px < int(aw); px++ {
			sumA := 0.0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/float64(samples)
					fy := float64(py) + (float64(sy)+0.5)/float64(samples)
					signedDistance := roundedBoxSDF(fx, fy, cx, cy, halfW, halfH, rr)
					borderDistance := math.Abs(signedDistance)
					core := math.Exp(-(borderDistance * borderDistance) / (2 * coreSigma * coreSigma))
					if borderDistance > 1.20*scale {
						core = 0
					}
					sumA += core
				}
			}
			avgA := sumA / float64(totalSamples)
			if avgA <= 0.002 {
				continue
			}
			i := py*canvas.Stride + px*4
			canvas.Pix[i] = 55
			canvas.Pix[i+1] = 61
			canvas.Pix[i+2] = 70
			canvas.Pix[i+3] = byte(math.Round(255 * math.Min(1, avgA)))
		}
	}
	return artworkFromNRGBA(canvas)
}
