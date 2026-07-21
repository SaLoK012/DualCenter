//go:build windows

package main

import (
	"testing"
	"time"
)

func TestFullscreenWarningShadowHasTransparentGutter(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()

	for _, scale := range []float64{0.5, 0.75, 1, 1.333, 2, 4} {
		state.scale = scale
		padding := float64(scaled(fullscreenWarningShadowPadding))
		extent := fullscreenWarningShadowExtent * scale
		if padding <= extent {
			t.Fatalf("scale %.3f: padding %.2f must exceed shadow extent %.2f", scale, padding, extent)
		}
	}
}

func TestFullscreenWarningUsesBatteryOverlayTint(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()
	state.scale = 1

	artwork, ok := makeFullscreenWarningCardArtwork(652, 158)
	if !ok {
		t.Fatal("não foi possível gerar a arte do aviso de fullscreen")
	}
	foundTint := false
	for i := 0; i+3 < len(artwork.pixels); i += 4 {
		blue := artwork.pixels[i]
		green := artwork.pixels[i+1]
		red := artwork.pixels[i+2]
		if blue > green && green > red && artwork.pixels[i+3] != 0 {
			foundTint = true
			break
		}
	}
	if !foundTint {
		t.Fatal("o aviso de fullscreen não usa a tonalidade do overlay de bateria")
	}
}

func TestFullscreenWarningTextIsCenteredWithCompactMargins(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()
	state.scale = 1

	c := rect{0, 0, 540, 124}
	title, body := fullscreenWarningTextRects(c)
	padding := scaled(fullscreenWarningShadowPadding)
	cardCenterTwice := padding + c.Right - padding
	if title.Left+title.Right != cardCenterTwice || body.Left+body.Right != cardCenterTwice {
		t.Fatalf("texto não está centralizado no cartão: título=%#v corpo=%#v", title, body)
	}
	if title.Left-padding != scaled(24) || c.Right-padding-title.Right != scaled(24) {
		t.Fatalf("margens horizontais inesperadas: título=%#v", title)
	}
	if fullscreenWarningTextFlags&DT_CENTER == 0 || fullscreenWarningTextFlags&DT_LEFT != 0 {
		t.Fatalf("alinhamento do aviso não está centralizado: flags=%#x", fullscreenWarningTextFlags)
	}
}

func TestMenuDockArtworkIsOnlyBlackGradient(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()
	state.scale = 1

	artwork, ok := makeMenuDockArtwork(1620, 380)
	if !ok {
		t.Fatal("não foi possível gerar a sombra do menu")
	}
	center := (int(artwork.h/2)*int(artwork.w) + int(artwork.w/2)) * 4
	if artwork.pixels[center+3] == 0 {
		t.Fatal("o centro da sombra ficou transparente")
	}
	for i := 0; i+3 < len(artwork.pixels); i += 4 {
		if artwork.pixels[i] != 0 || artwork.pixels[i+1] != 0 || artwork.pixels[i+2] != 0 {
			t.Fatalf("pixel %d contém cor; a base deve usar somente preto", i/4)
		}
	}
	for _, corner := range []int{0, int(artwork.w-1) * 4, (int(artwork.h-1) * int(artwork.w)) * 4, (int(artwork.h*artwork.w) - 1) * 4} {
		if artwork.pixels[corner+3] != 0 {
			t.Fatalf("canto da sombra não terminou em transparência: alpha=%d", artwork.pixels[corner+3])
		}
	}
}

func TestBatteryOverlayAccentIsBlue(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()
	state.scale = 1

	artwork, ok := batteryIconArtworkAccent(100, true, 35, 18, batteryOverlayAccent)
	if !ok {
		t.Fatal("não foi possível gerar a bateria do overlay")
	}
	foundBlue := false
	for i := 0; i+3 < len(artwork.pixels); i += 4 {
		blue := artwork.pixels[i]
		green := artwork.pixels[i+1]
		red := artwork.pixels[i+2]
		if blue > green && green > red && artwork.pixels[i+3] != 0 {
			foundBlue = true
			break
		}
	}
	if !foundBlue {
		t.Fatal("a bateria do overlay não contém o preenchimento azul esperado")
	}
}

func TestSelectionTransitionUsesShortEaseOut(t *testing.T) {
	originalStart := state.selectionStartedAt
	defer func() { state.selectionStartedAt = originalStart }()

	start := time.Unix(100, 0)
	state.selectionStartedAt = start
	if frame, active := selectionFrameAtLocked(start); frame != 0 || !active {
		t.Fatalf("início da transição = (%d, %t); esperado (0, true)", frame, active)
	}
	if frame, active := selectionFrameAtLocked(start.Add(selectionTransitionDuration / 2)); frame < selectionTransitionFrames/2 || !active {
		t.Fatalf("meio da transição = (%d, %t); ease-out não avançou como esperado", frame, active)
	}
	if frame, active := selectionFrameAtLocked(start.Add(selectionTransitionDuration)); frame != selectionTransitionFrames || active {
		t.Fatalf("fim da transição = (%d, %t); esperado (%d, false)", frame, active, selectionTransitionFrames)
	}
}

func TestProfessionalPanelSpacingRemainsBalanced(t *testing.T) {
	originalScale := state.scale
	defer func() { state.scale = originalScale }()
	state.scale = 1

	first := panelRect(0)
	second := panelRect(1)
	last := panelRect(4)
	if got := second.Left - first.Right; got != 12 {
		t.Fatalf("espaço entre cartões = %d; esperado 12", got)
	}
	if first.Left != 16 || last.Right != 1374 {
		t.Fatalf("margens do conjunto inesperadas: primeiro=%#v último=%#v", first, last)
	}
}

func TestCustomUIIconsLoadAndScale(t *testing.T) {
	for _, path := range []string{"assets/icon_gamebar.png", "assets/icon_usb.png"} {
		mask, ok := loadUIIconMask(path)
		if !ok {
			t.Fatalf("não foi possível carregar %s", path)
		}
		artwork, ok := makeUIIconArtwork(mask, 21)
		if !ok || artwork.w < 1 || artwork.h < 1 || artwork.w > 21 || artwork.h > 21 {
			t.Fatalf("%s foi dimensionado incorretamente: %#v", path, artwork)
		}
		visible := false
		for i := 0; i+3 < len(artwork.pixels); i += 4 {
			a := artwork.pixels[i+3]
			if artwork.pixels[i] != a || artwork.pixels[i+1] != a || artwork.pixels[i+2] != a {
				t.Fatalf("%s não foi convertido para branco pré-multiplicado", path)
			}
			visible = visible || a != 0
		}
		if !visible {
			t.Fatalf("%s ficou totalmente transparente", path)
		}
	}
}
