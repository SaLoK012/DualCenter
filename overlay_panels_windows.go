// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"dualcenter/internal/version"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func panelRect(i int) rect {
	// Mais espaço entre cartões sem ampliar a janela: a largura individual foi
	// compensada para preservar o encaixe exato das cinco abas em 1080p e 4K.
	left := menuShadowPaddingX() + scaled(16)
	gap := scaled(12)
	w := scaled(262)
	top := menuShadowPaddingY() + scaled(8)
	h := scaled(305)
	return rect{left + int32(i)*(w+gap), top, left + int32(i)*(w+gap) + w, top + h}
}

// A janela ganha somente uma margem transparente para a sombra. O conteúdo
// recebe o mesmo deslocamento, portanto as abas permanecem no mesmo ponto da tela.
func menuShadowPaddingX() int32 { return scaled(24) }
func menuShadowPaddingY() int32 { return scaled(44) }

func paintMenu(hdc uintptr, c rect) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.tabOrder) != tabCount {
		state.tabOrder = defaultTabOrder()
	}
	refreshSelectionTransitionLocked(time.Now())
	ensureSelectedTabVisibleLocked()
	menuDockSurface(hdc, c)
	for slot := 0; slot < 5; slot++ {
		r := panelRect(slot)
		orderPos := state.visibleStart + slot
		if orderPos >= len(state.tabOrder) {
			continue
		}
		tab := state.tabOrder[orderPos]
		fullPanelSelected := tab == state.menuPanel && (state.organizing ||
			tab == tabBattery ||
			(tab == tabGames && !state.gamesFocused) ||
			(tab == tabVolume && state.row[tabVolume] == 0))
		panelSurface(hdc, r, fullPanelSelected)
		switch tab {
		case tabVolume:
			paintVolumePanel(hdc, r)
		case tabEnergy:
			paintEnergyPanel(hdc, r)
		case tabModeGame:
			paintModeGamePanel(hdc, r)
		case tabSettings:
			paintSettingsPanel(hdc, r)
		case tabBattery:
			paintBatteryInfoPanel(hdc, r)
		case tabGames:
			paintGamesPanel(hdc, r)
		}
	}

	cy := panelRect(0).Top + (panelRect(0).Bottom-panelRect(0).Top)/2
	arrowGap := scaled(14)
	if state.visibleStart > 0 {
		drawCarouselArrow(hdc, panelRect(0).Left-arrowGap, cy, -1)
	}
	if state.visibleStart+5 < tabCount {
		drawCarouselArrow(hdc, panelRect(4).Right+arrowGap, cy, 1)
	}

	// A legenda fixa foi removida para manter o menu limpo e intuitivo.
}

func drawCarouselArrow(hdc uintptr, cx, cy int32, direction int) {
	// Três passagens mantêm o halo controlado e uma linha central bem definida.
	drawCarouselArrowStroke(hdc, cx, cy, direction, scaledStroke(5), rgb(4, 27, 48))
	drawCarouselArrowStroke(hdc, cx, cy, direction, scaledStroke(3), rgb(12, 78, 132))
	drawCarouselArrowStroke(hdc, cx, cy, direction, scaledStroke(1), rgb(92, 190, 255))
}

func drawCarouselArrowStroke(hdc uintptr, cx, cy int32, direction int, width int32, color uintptr) {
	pen := cachedPen(width, color)
	old, _, _ := pSelectObject.Call(hdc, pen)
	d := scaled(7)
	if direction < 0 {
		pMoveToEx.Call(hdc, uintptr(int64(cx+d)), uintptr(int64(cy-d)), 0)
		pLineTo.Call(hdc, uintptr(int64(cx)), uintptr(int64(cy)))
		pLineTo.Call(hdc, uintptr(int64(cx+d)), uintptr(int64(cy+d)))
	} else {
		pMoveToEx.Call(hdc, uintptr(int64(cx-d)), uintptr(int64(cy-d)), 0)
		pLineTo.Call(hdc, uintptr(int64(cx)), uintptr(int64(cy)))
		pLineTo.Call(hdc, uintptr(int64(cx-d)), uintptr(int64(cy+d)))
	}
	pSelectObject.Call(hdc, old)
}

func header(hdc uintptr, r rect, title string) {
	textFont(hdc, title, rect{r.Left, r.Top, r.Right, r.Top + scaled(48)}, rgb(247, 248, 250), scaled(15), FW_SEMIBOLD, panelTitleFont, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	line(hdc, r.Left+scaled(12), r.Top+scaled(48), r.Right-scaled(12), r.Top+scaled(48), rgb(42, 47, 55))
}

func rowRect(r rect, index int) rect {
	// Quatro posições idênticas, centralizadas entre o cabeçalho e a base.
	// Volume e Controle usam apenas as duas últimas posições para manter seus
	// blocos principais grandes e perfeitamente alinhados.
	top := r.Top + scaled(62) + int32(index)*scaled(54)
	return rect{r.Left + scaled(8), top, r.Right - scaled(8), top + scaled(52)}
}

func drawRow(hdc uintptr, r rect, row int, glyph, label, status string, active bool) {
	rr := rowRect(r, row)
	drawRowInRect(hdc, rr, glyph, label, status, active)
}

const (
	gameBarIcon     = "@gamebar-image"
	usbIcon         = "@usb-image"
	windowsFlagIcon = "@windows-flag"
)

func drawWindowsFlagIcon(hdc uintptr, r rect, color uintptr) {
	w := scaled(8)
	h := scaled(8)
	gap := scaled(2)
	left := r.Left + (r.Right-r.Left-(w*2+gap))/2
	top := r.Top + (r.Bottom-r.Top-(h*2+gap))/2
	fillOpaqueRect(hdc, rect{left, top, left + w, top + h}, color)
	fillOpaqueRect(hdc, rect{left + w + gap, top, left + w*2 + gap, top + h}, color)
	fillOpaqueRect(hdc, rect{left, top + h + gap, left + w, top + h*2 + gap}, color)
	fillOpaqueRect(hdc, rect{left + w + gap, top + h + gap, left + w*2 + gap, top + h*2 + gap}, color)
}

func drawRowInRect(hdc uintptr, rr rect, glyph, label, status string, active bool) {
	if active {
		selectionNeon(hdc, rr, scaled(6))
	}
	iconTint := rgb(232, 235, 240)
	if active {
		iconTint = rgb(72, 172, 248)
	}
	iconRect := rect{rr.Left + scaled(6), rr.Top, rr.Left + scaled(42), rr.Bottom}
	if glyph == gameBarIcon || glyph == usbIcon {
		if !drawUIIcon(hdc, glyph, iconRect, iconTint) {
			fallback := "X"
			if glyph == usbIcon {
				fallback = "\uE88E"
			}
			iconColor(hdc, fallback, iconRect, scaled(20), iconTint)
		}
	} else if glyph == windowsFlagIcon {
		drawWindowsFlagIcon(hdc, iconRect, iconTint)
	} else {
		iconColor(hdc, glyph, iconRect, scaled(20), iconTint)
	}
	text(hdc, label, rect{rr.Left + scaled(46), rr.Top, rr.Right - scaled(66), rr.Bottom}, rgb(242, 244, 247), scaled(12), FW_MEDIUM, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	if status != "" {
		col := rgb(174, 181, 191)
		if status == "Ligado" || status == "Ligada" {
			col = rgb(48, 158, 242)
		}
		text(hdc, status, rect{rr.Right - scaled(95), rr.Top, rr.Right - scaled(6), rr.Bottom}, col, scaled(11), FW_NORMAL, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
}

func drawRowSeparators(hdc uintptr, r rect, count, selected int) {
	for i := 0; i < count-1; i++ {
		// A divisória imediatamente acima ou abaixo da opção selecionada some,
		// deixando o contorno neon limpo e sem uma reta encostando nos cantos.
		if selected == i || selected == i+1 {
			continue
		}
		rr := rowRect(r, i)
		line(hdc, rr.Left+scaled(12), rr.Bottom, rr.Right-scaled(12), rr.Bottom, rgb(34, 37, 43))
	}
}

func paintVolumePanel(hdc uintptr, r rect) {
	header(hdc, r, "Volume")
	if state.audioPicker {
		paintAudioOutputPicker(hdc, r)
		return
	}
	vol, mute := getVolume()
	rr1 := rowRect(r, 2)
	rr2 := rowRect(r, 3)
	main := rowRect(r, 0)
	// Elimina a antiga sobreposição de oito pixels entre o bloco principal e
	// a opção Mudo. O contorno completo do painel é desenhado em paintMenu.
	main.Bottom = rr1.Top - scaled(2)
	mainActive := state.menuPanel == tabVolume && !state.organizing && state.row[tabVolume] == 0
	mainIconColor := rgb(232, 235, 240)
	if mainActive {
		mainIconColor = rgb(72, 172, 248)
	}
	iconColor(hdc, "\uE767", rect{main.Left + scaled(4), main.Top + scaled(6), main.Left + scaled(48), main.Top + scaled(62)}, scaled(24), mainIconColor)
	text(hdc, fmt.Sprintf("%d%%", int(vol*100+0.5)), rect{main.Left, main.Top + scaled(6), main.Right, main.Top + scaled(62)}, rgb(250, 250, 252), scaled(26), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(hdc, "−", rect{main.Left + scaled(6), main.Top + scaled(58), main.Left + scaled(38), main.Bottom}, rgb(245, 245, 248), scaled(19), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(hdc, "+", rect{main.Right - scaled(38), main.Top + scaled(58), main.Right - scaled(6), main.Bottom}, rgb(245, 245, 248), scaled(19), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	segments := 20
	filled := int(vol*float32(segments) + 0.5)
	segmentW := scaled(5)
	segmentStep := scaled(7)
	barW := int32(segments-1)*segmentStep + segmentW
	sx := main.Left + (main.Right-main.Left-barW)/2
	sy := main.Top + scaled(84)
	for i := 0; i < segments; i++ {
		col := rgb(55, 58, 64)
		if i < filled {
			col = rgb(33, 147, 255)
		}
		x := sx + int32(i)*segmentStep
		fillRect(hdc, rect{x, sy, x + segmentW, sy + scaled(11)}, col)
	}
	if state.volumeAdjust {
		// O aviso fica no espaço livre entre a porcentagem e a barra, sem
		// encobrir os segmentos de volume.
		text(hdc, "AJUSTE ATIVO", rect{main.Left, main.Top + scaled(63), main.Right, main.Top + scaled(81)}, rgb(35, 153, 255), scaled(8), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}

	rr1Active := state.menuPanel == tabVolume && !state.organizing && state.row[tabVolume] == 1
	if rr1Active {
		selectionNeon(hdc, rr1, scaled(6))
	}
	rr1IconColor := rgb(232, 235, 240)
	if rr1Active {
		rr1IconColor = rgb(72, 172, 248)
	}
	iconColor(hdc, "\uE74F", rect{rr1.Left + scaled(6), rr1.Top, rr1.Left + scaled(42), rr1.Bottom}, scaled(20), rr1IconColor)
	text(hdc, "Mudo", rect{rr1.Left + scaled(46), rr1.Top, rr1.Right - scaled(70), rr1.Bottom}, rgb(242, 244, 247), scaled(12), FW_MEDIUM, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	ms := "Desligado"
	if mute {
		ms = "Ligado"
	}
	text(hdc, ms, rect{rr1.Right - scaled(92), rr1.Top, rr1.Right - scaled(6), rr1.Bottom}, rgb(174, 181, 191), scaled(11), FW_NORMAL, DT_RIGHT|DT_VCENTER|DT_SINGLELINE)

	rr2Active := state.menuPanel == tabVolume && !state.organizing && state.row[tabVolume] == 2
	if rr2Active {
		selectionNeon(hdc, rr2, scaled(6))
	}
	rr2IconColor := rgb(232, 235, 240)
	if rr2Active {
		rr2IconColor = rgb(72, 172, 248)
	}
	iconColor(hdc, "\uE7F6", rect{rr2.Left + scaled(6), rr2.Top, rr2.Left + scaled(42), rr2.Bottom}, scaled(20), rr2IconColor)
	text(hdc, "Saída de áudio", rect{rr2.Left + scaled(46), rr2.Top, rr2.Right - scaled(92), rr2.Bottom}, rgb(242, 244, 247), scaled(12), FW_MEDIUM, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	name := state.audioName
	if name == "" {
		name = "Padrão"
	}
	text(hdc, name, rect{rr2.Right - scaled(118), rr2.Top, rr2.Right - scaled(6), rr2.Bottom}, rgb(174, 181, 191), scaled(11), FW_NORMAL, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	selected := -1
	if state.menuPanel == tabVolume && !state.organizing {
		selected = state.row[tabVolume]
	}
	if selected != 0 && selected != 1 {
		line(hdc, main.Left+scaled(12), main.Bottom+scaled(1), main.Right-scaled(12), main.Bottom+scaled(1), rgb(34, 37, 43))
	}
	if selected != 1 && selected != 2 {
		line(hdc, rr1.Left+scaled(12), rr1.Bottom, rr1.Right-scaled(12), rr1.Bottom, rgb(34, 37, 43))
	}
}

func paintAudioOutputPicker(hdc uintptr, r rect) {
	text(hdc, "Saídas disponíveis", rect{r.Left + scaled(12), r.Top + scaled(50), r.Right - scaled(12), r.Top + scaled(82)}, rgb(180, 185, 195), scaled(11), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if len(state.audioOutputs) == 0 {
		text(hdc, "Nenhuma saída encontrada", rect{r.Left + scaled(12), r.Top + scaled(100), r.Right - scaled(12), r.Top + scaled(155)}, rgb(180, 185, 195), scaled(10), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		return
	}
	start := 0
	if state.audioOutputIndex > 3 {
		start = state.audioOutputIndex - 3
	}
	end := start + 4
	if end > len(state.audioOutputs) {
		end = len(state.audioOutputs)
	}
	for i := start; i < end; i++ {
		rr := rect{r.Left + scaled(8), r.Top + scaled(85) + int32(i-start)*scaled(49), r.Right - scaled(8), r.Top + scaled(132) + int32(i-start)*scaled(49)}
		if i == state.audioOutputIndex {
			selectionNeon(hdc, rr, scaled(10))
		}
		icon(hdc, "\uE7F6", rect{rr.Left + scaled(4), rr.Top, rr.Left + scaled(42), rr.Bottom}, scaled(19))
		text(hdc, state.audioOutputs[i].Name, rect{rr.Left + scaled(44), rr.Top, rr.Right - scaled(50), rr.Bottom}, rgb(245, 245, 248), scaled(10), FW_NORMAL, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		if i == currentAudioOutputIndex(state.audioOutputs) {
			text(hdc, "Ativa", rect{rr.Right - scaled(48), rr.Top, rr.Right - scaled(4), rr.Bottom}, rgb(0, 145, 255), scaled(9), FW_NORMAL, DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		}
	}
}

func paintEnergyPanel(hdc uintptr, r rect) {
	header(hdc, r, "Energia")
	active := state.menuPanel == tabEnergy && !state.organizing
	drawRow(hdc, r, 0, "\uE7E8", "Desligar computador", confirmStatus("shutdown"), active && state.row[tabEnergy] == 0)
	drawRow(hdc, r, 1, "\uE895", "Reiniciar computador", confirmStatus("restart"), active && state.row[tabEnergy] == 1)
	drawRow(hdc, r, 2, "\uE708", "Suspender computador", confirmStatus("suspend"), active && state.row[tabEnergy] == 2)
	drawRow(hdc, r, 3, "\uE711", "Cancelar desligamento", "", active && state.row[tabEnergy] == 3)
	selected := -1
	if active {
		selected = state.row[tabEnergy]
	}
	drawRowSeparators(hdc, r, 4, selected)
}

func confirmStatus(a string) string {
	if state.confirmAction == a && time.Now().Before(state.confirmUntil) {
		return "X confirmar"
	}
	return ""
}

func paintModeGamePanel(hdc uintptr, r rect) {
	header(hdc, r, "Preferências")
	active := state.menuPanel == tabModeGame && !state.organizing
	drawRow(hdc, r, 0, "\uE706", "Overlay Bateria", onoff(batteryOverlayEnabled(state.hideBatteryOverlay)), active && state.row[tabModeGame] == 0)
	drawRow(hdc, r, 1, "\uE850", "Aviso de bateria", onoff(state.batteryAlerts), active && state.row[tabModeGame] == 1)
	drawRow(hdc, r, 2, "\uE72E", "Bloquear desligamento", onoff(state.blockShutdown), active && state.row[tabModeGame] == 2)
	drawRow(hdc, r, 3, windowsFlagIcon, "Ícone na barra de tarefas", onoff(state.showTaskbarIcon), active && state.row[tabModeGame] == 3)
	selected := -1
	if active {
		selected = state.row[tabModeGame]
	}
	drawRowSeparators(hdc, r, 4, selected)
}

func paintSettingsPanel(hdc uintptr, r rect) {
	header(hdc, r, "Sistema")
	active := state.menuPanel == tabSettings && !state.organizing
	drawRow(hdc, r, 0, "\uEA37", "Iniciar com o Windows", onoff(state.startupEnabled), active && state.row[tabSettings] == 0)
	drawRow(hdc, r, 1, gameBarIcon, "Game Bar", onoffFeminine(state.gameBarEnabled), active && state.row[tabSettings] == 1)
	drawRow(hdc, r, 2, "\uE823", "Tempo do aviso", fmt.Sprintf("%d segundos", state.warningSeconds), active && state.row[tabSettings] == 2)
	drawRow(hdc, r, 3, "\uE946", "Sobre o DualCenter", version.Display(), active && state.row[tabSettings] == 3)
	selected := -1
	if active {
		selected = state.row[tabSettings]
	}
	drawRowSeparators(hdc, r, 4, selected)
}

func gameTileRect(r rect, cell int) rect {
	content := rect{r.Left + scaled(8), r.Top + scaled(56), r.Right - scaled(8), r.Bottom - scaled(8)}
	gap := scaled(3)
	w := (content.Right - content.Left - gap*4) / 5
	h := (content.Bottom - content.Top - gap*4) / 5
	col := int32(cell % 5)
	row := int32(cell / 5)
	left := content.Left + col*(w+gap)
	top := content.Top + row*(h+gap)
	return rect{left, top, left + w, top + h}
}

func hresultSucceeded(value uintptr) bool {
	return int32(uint32(value)) >= 0
}

// imageListMethod returns a COM method address from an IImageList vtable.
func imageListMethod(imageList unsafe.Pointer, index uintptr) uintptr {
	if imageList == nil {
		return 0
	}
	vtable := *(*unsafe.Pointer)(imageList)
	if vtable == nil {
		return 0
	}
	return *(*uintptr)(unsafe.Add(vtable, index*unsafe.Sizeof(uintptr(0))))
}

func releaseImageList(imageList unsafe.Pointer) {
	method := imageListMethod(imageList, 2) // IUnknown::Release
	if method != 0 {
		syscall.SyscallN(method, uintptr(imageList))
	}
}

func iconFromSystemImageList(iconIndex int32, listSize int32) uintptr {
	if err := pSHGetImageList.Find(); err != nil {
		return 0
	}
	var imageList unsafe.Pointer
	hr, _, _ := pSHGetImageList.Call(
		uintptr(listSize),
		uintptr(unsafe.Pointer(&iidIImageList)),
		uintptr(unsafe.Pointer(&imageList)),
	)
	if !hresultSucceeded(hr) || imageList == nil {
		return 0
	}
	defer releaseImageList(imageList)

	getIcon := imageListMethod(imageList, 10) // IImageList::GetIcon
	if getIcon == 0 {
		return 0
	}
	var iconHandle uintptr
	hr, _, _ = syscall.SyscallN(
		getIcon,
		uintptr(imageList),
		uintptr(iconIndex),
		ILD_TRANSPARENT,
		uintptr(unsafe.Pointer(&iconHandle)),
	)
	if !hresultSucceeded(hr) || iconHandle == 0 {
		return 0
	}
	return iconHandle
}

func highResolutionGameIcon(path string) uintptr {
	var info shFileInfo
	ret, _, _ := pSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(utf16Ptr(path))),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		SHGFI_SYSICONINDEX,
	)
	if ret == 0 || info.IIcon < 0 {
		return 0
	}

	// SHIL_JUMBO fornece a arte de até 256×256 do cache do Explorer. Isso
	// evita ampliar o ícone legado de 32×32 retornado por SHGFI_LARGEICON.
	if iconHandle := iconFromSystemImageList(info.IIcon, SHIL_JUMBO); iconHandle != 0 {
		return iconHandle
	}
	return iconFromSystemImageList(info.IIcon, SHIL_EXTRALARGE)
}

func gameIconForPath(path string) uintptr {
	key := strings.ToLower(filepath.Clean(path))
	if iconHandle, ok := gameIconCache[key]; ok {
		return iconHandle
	}

	if iconHandle := highResolutionGameIcon(path); iconHandle != 0 {
		gameIconCache[key] = iconHandle
		return iconHandle
	}

	// Compatibilidade para sistemas em que SHGetImageList não esteja disponível.
	var info shFileInfo
	ret, _, _ := pSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(utf16Ptr(path))),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		SHGFI_ICON|SHGFI_LARGEICON,
	)
	if ret == 0 || info.HIcon == 0 {
		gameIconCache[key] = 0
		return 0
	}
	gameIconCache[key] = info.HIcon
	return info.HIcon
}

func drawGameExecutableIcon(hdc uintptr, path string, r rect) bool {
	iconHandle := gameIconForPath(path)
	if iconHandle == 0 {
		return false
	}
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	pDrawIconEx.Call(
		hdc,
		uintptr(int64(r.Left)),
		uintptr(int64(r.Top)),
		iconHandle,
		uintptr(w),
		uintptr(h),
		0,
		0,
		DI_NORMAL,
	)
	return true
}

func paintGamesPanel(hdc uintptr, r rect) {
	header(hdc, r, "Jogos")
	total := len(state.games) + 1
	selected := state.row[tabGames]
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
		state.row[tabGames] = selected
	}

	// Mostra vinte e cinco mosaicos (5×5) por página. X ou Triângulo abre o
	// menu contextual pequeno; abrir, fechar e remover nunca substituem a grade.
	start := (selected / 25) * 25

	for cell := 0; cell < 25; cell++ {
		index := start + cell
		if index >= total {
			break
		}
		tile := gameTileRect(r, cell)
		active := state.menuPanel == tabGames && state.gamesFocused && !state.organizing && !state.gameMenuOpen && selected == index

		iconSize := tile.Right - tile.Left - scaled(3)
		if h := tile.Bottom - tile.Top - scaled(3); h < iconSize {
			iconSize = h
		}
		if iconSize < scaled(12) {
			iconSize = scaled(12)
		}
		iconLeft := tile.Left + (tile.Right-tile.Left-iconSize)/2
		iconTop := tile.Top + (tile.Bottom-tile.Top-iconSize)/2
		iconRect := rect{iconLeft, iconTop, iconLeft + iconSize, iconTop + iconSize}
		if active {
			selectionNeon(hdc, iconRect, scaled(2))
		}

		if index == len(state.games) {
			iconColor(hdc, "\uE710", iconRect, scaled(19), rgb(142, 148, 158))
			continue
		}

		game := state.games[index]
		if !drawGameExecutableIcon(hdc, game.Path, iconRect) {
			icon(hdc, "\uE7FC", iconRect, scaled(21))
		}

		if index < len(state.gameRunning) && state.gameRunning[index] {
			dot := rect{iconRect.Right - scaled(7), iconRect.Top + scaled(2), iconRect.Right - scaled(2), iconRect.Top + scaled(7)}
			rounded(hdc, dot, rgb(36, 154, 255), rgb(36, 154, 255), scaled(3), 1)
		}
	}

	if state.gameMenuOpen && state.gameMenuGame >= start && state.gameMenuGame < start+25 {
		paintGameActionMiniMenu(hdc, r, start)
	}
}

func gameActionGlyph(action string) string {
	switch action {
	case "Abrir jogo":
		return "\uE768"
	case "Fechar jogo":
		return "\uE711"
	case "Remover jogo":
		return "\uE74D"
	default:
		return "\uE7FC"
	}
}

func paintGameActionMiniMenu(hdc uintptr, panel rect, pageStart int) {
	index := state.gameMenuGame
	if index < 0 || index >= len(state.games) {
		return
	}
	actions := gameMenuActionsLocked(index)
	if len(actions) == 0 {
		return
	}
	selected := clamp(state.gameMenuSelection, 0, len(actions)-1)
	state.gameMenuSelection = selected

	cell := index - pageStart
	anchor := gameTileRect(panel, cell)
	// O menu foi reduzido sem comprometer os rótulos nem a confirmação.
	// A proporção compacta aproxima as ações do ícone selecionado e ocupa menos
	// espaço da grade, mantendo a mesma linguagem visual das opções internas.
	menuW := scaled(142)
	rowH := scaled(40)
	menuH := int32(len(actions)) * rowH
	left := anchor.Right + scaled(4)
	if left+menuW > panel.Right-scaled(5) {
		left = anchor.Left - menuW - scaled(4)
	}
	if left < panel.Left+scaled(5) {
		left = panel.Left + scaled(5)
	}
	top := anchor.Top + (anchor.Bottom-anchor.Top-menuH)/2
	if top+menuH > panel.Bottom-scaled(5) {
		top = panel.Bottom - scaled(5) - menuH
	}
	if top < panel.Top+scaled(52) {
		top = panel.Top + scaled(52)
	}

	// O fundo recebe alpha 255 antes da composição dos halos. Assim, as zonas
	// entre as opções permanecem totalmente opacas e nunca revelam a área de
	// trabalho, mesmo quando o brilho da opção ativa atravessa a divisória.
	fillOpaqueRect(hdc, rect{left, top, left + menuW, top + menuH}, rgb(12, 15, 20))

	now := time.Now()
	for i, action := range actions {
		option := rect{left, top + int32(i)*rowH, left + menuW, top + int32(i+1)*rowH}
		if i == selected {
			// A opção ativa conserva o mesmo SDF, linha e brilho do restante do menu,
			// apenas em uma geometria menor e mais equilibrada.
			compactSelectedOption(hdc, option, scaled(5))
		}

		confirming := (action == "Fechar jogo" && state.confirmAction == fmt.Sprintf("closegame:%d", index) && now.Before(state.confirmUntil)) ||
			(action == "Remover jogo" && state.confirmAction == fmt.Sprintf("removegame:%d", index) && now.Before(state.confirmUntil))
		iconColor(hdc, gameActionGlyph(action), rect{option.Left + scaled(2), option.Top, option.Left + scaled(34), option.Bottom}, scaled(16), rgb(245, 245, 248))
		if confirming {
			text(hdc, action, rect{option.Left + scaled(35), option.Top, option.Right - scaled(4), option.Top + scaled(23)}, rgb(245, 245, 248), scaled(8), FW_NORMAL, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(hdc, "X confirmar", rect{option.Left + scaled(35), option.Top + scaled(17), option.Right - scaled(4), option.Bottom}, rgb(245, 185, 45), scaled(7), FW_NORMAL, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		} else {
			text(hdc, action, rect{option.Left + scaled(35), option.Top, option.Right - scaled(4), option.Bottom}, rgb(245, 245, 248), scaled(9), FW_NORMAL, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		}
	}

	// As opções não selecionadas permanecem limpas sobre o fundo do painel.
	// A divisória não toca a opção ativa para o halo não ser cortado.
	for i := 0; i < len(actions)-1; i++ {
		if selected == i || selected == i+1 {
			continue
		}
		y := top + int32(i+1)*rowH
		line(hdc, left+scaled(10), y, left+menuW-scaled(10), y, rgb(34, 37, 43))
	}
}

func paintBatteryInfoPanel(hdc uintptr, r rect) {
	header(hdc, r, "Controle")
	// Mantém a arte original sem distorção, um pouco menor e com respiro igual
	// ao restante do painel.
	drawController(hdc, rect{r.Left + scaled(18), r.Top + scaled(68), r.Left + scaled(122), r.Top + scaled(151)})
	// Ícone e porcentagem formam um único bloco, centralizado verticalmente.
	drawBatteryIcon(hdc, r.Left+scaled(143), r.Top+scaled(91), state.batteryPercent, state.batteryKnown)
	pct := "--%"
	if state.batteryKnown {
		pct = fmt.Sprintf("%d%%", state.batteryPercent)
	}
	text(hdc, pct, rect{r.Left + scaled(177), r.Top + scaled(72), r.Right - scaled(8), r.Top + scaled(126)}, rgb(250, 250, 252), scaled(22), FW_SEMIBOLD, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	st := "Em uso"
	if !state.batteryKnown {
		st = "Indisponível"
	} else if state.charging {
		st = "Carregando"
	}
	icon(hdc, "\uE945", rect{r.Left + scaled(144), r.Top + scaled(122), r.Left + scaled(174), r.Top + scaled(158)}, scaled(17))
	text(hdc, st, rect{r.Left + scaled(177), r.Top + scaled(122), r.Right - scaled(8), r.Top + scaled(158)}, rgb(245, 245, 248), scaled(10), FW_NORMAL, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	rr1 := rowRect(r, 2)
	rr2 := rowRect(r, 3)
	line(hdc, rr1.Left+scaled(12), rr1.Top-scaled(2), rr1.Right-scaled(12), rr1.Top-scaled(2), rgb(34, 37, 43))
	drawRowInRect(hdc, rr1, usbIcon, "Conexão", state.connection, false)
	line(hdc, rr1.Left+scaled(12), rr1.Bottom, rr1.Right-scaled(12), rr1.Bottom, rgb(34, 37, 43))
	drawRowInRect(hdc, rr2, "\uE850", "Status da bateria", st, false)
}

func onoff(v bool) string {
	if v {
		return "Ligado"
	}
	return "Desligado"
}

func onoffFeminine(v bool) string {
	if v {
		return "Ligada"
	}
	return "Desligada"
}
