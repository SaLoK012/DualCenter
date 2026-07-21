// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"math"
	"strings"
	"time"
	"unsafe"
)

func showBatteryLocked() {
	if state.circleReleasePending {
		return
	}
	state.overlayMode = overlayBattery
	state.toastHideAt = time.Now().Add(3 * time.Second)
	pShowWindow.Call(mainWindow, SW_HIDE)
	repositionCurrentOverlay(false)
	state.pendingOverlayShow = SW_SHOWNOACTIVATE
	updateTimerIntervalLocked()
}

func showMessageLocked(t, b string, d time.Duration) {
	if state.circleReleasePending {
		return
	}
	state.overlayMode = overlayMessage
	state.messageTitle = t
	state.messageBody = b
	state.toastHideAt = time.Now().Add(d)
	pShowWindow.Call(mainWindow, SW_HIDE)
	repositionCurrentOverlay(false)
	state.pendingOverlayShow = SW_SHOWNOACTIVATE
	updateTimerIntervalLocked()
}

func showMenuLocked(device uintptr) {
	if state.circleReleasePending {
		return
	}
	state.pendingOverlayShow = 0
	state.toastHideAt = time.Time{}
	state.messageBody = ""
	state.savedForeground = overlayTargetWindow()
	// O menu precisa virar foreground para impedir que jogos em janela/sem bordas
	// continuem recebendo comandos do controle atrás dele. Em fullscreen exclusivo,
	// também forçamos foreground/topmost: é o único caminho Win32 simples para o
	// menu aparecer por cima sem deixar o jogo comandável em segundo plano.
	state.overlayMode = overlayMenu
	if len(state.tabOrder) != tabCount {
		state.tabOrder = defaultTabOrder()
	}
	state.menuPos = tabPositionLocked(state.menuPanel)
	ensureSelectedTabVisibleLocked()
	state.lastGamePoll = time.Time{}
	state.audioPicker = false
	state.volumeAdjust = false
	state.gameMenuOpen = false
	state.gameMenuGame = -1
	state.gameMenuSelection = 0
	state.gamesFocused = false
	state.gameDialogOpen = false
	state.confirmAction = ""
	state.selectionKeyValid = false
	state.selectionStartedAt = time.Time{}
	state.selectionFrame = selectionTransitionFrames
	state.menuDevice = device
	pShowWindow.Call(mainWindow, SW_HIDE)
	repositionCurrentOverlay(false)
	state.lastAudioPoll = time.Now()
	// Guarde a posição antes de ocultar o cursor. A centralização fica para o
	// instante em que o overlay já estiver visível e ativo; movê-lo enquanto a
	// janela ainda estava escondida fazia o jogo por baixo receber esse movimento.
	saveCursorPositionLocked()
	blockPhysicalInputLocked()
	hideCursorLocked()
	captureMenuFocusLocked()
	state.pendingOverlayShow = SW_SHOW
	updateTimerIntervalLocked()
}

func closeMenuLocked() {
	if state.overlayMode != overlayMenu {
		return
	}
	finalizeHideOverlayLocked()
}

func closeMenuAfterCircleLocked(device uintptr) {
	if state.overlayMode != overlayMenu {
		return
	}
	state.circleReleasePending = true
	state.circleReleaseDevice = device
	finalizeHideOverlayLocked()
}

func finalizeHideOverlayLocked() {
	if state.organizing && len(state.organizeOriginal) == tabCount {
		currentTab := state.menuPanel
		state.tabOrder = append([]int(nil), state.organizeOriginal...)
		state.menuPos = tabPositionLocked(currentTab)
		ensureSelectedTabVisibleLocked()
	}
	state.overlayMode = overlayHidden
	state.menuDevice = 0
	state.organizing = false
	state.organizeOriginal = nil
	state.gameMenuOpen = false
	state.gameMenuGame = -1
	state.gameMenuSelection = 0
	state.gamesFocused = false
	state.gameDialogOpen = false
	state.confirmAction = ""
	state.selectionKeyValid = false
	state.selectionStartedAt = time.Time{}
	state.selectionFrame = selectionTransitionFrames
	state.toastHideAt = time.Time{}
	state.messageBody = ""
	state.pendingOverlayShow = 0
	if state.circleReleasePending {
		// Remova o menu da tela imediatamente, mas mantenha a janela do DualCenter
		// como foreground até a soltura física do Círculo. Assim o mesmo comando
		// que fecha o menu nunca chega ao jogo por baixo.
		offscreen := int32(-32000)
		pSetWindowPos.Call(
			mainWindow,
			HWND_TOPMOST,
			uintptr(int64(offscreen)), uintptr(int64(offscreen)), 1, 1,
			SWP_NOACTIVATE|SWP_SHOWWINDOW,
		)
		releaseLayeredFrame()
		updateTimerIntervalLocked()
		return
	}
	pShowWindow.Call(mainWindow, SW_HIDE)
	// A superfície do menu é o maior bloco gráfico do processo (cerca de
	// 2,5 MiB em 1080p e 9,4 MiB em 4K). Quando o overlay está oculto ela não
	// traz benefício, então devolva-a imediatamente ao Windows.
	releaseLayeredFrame()
	restoreOverlayInputLocked(true)
	updateTimerIntervalLocked()
}

func completeCircleReleaseLocked(restoreFocus bool) {
	if !state.circleReleasePending {
		return
	}
	state.circleReleasePending = false
	state.circleReleaseDevice = 0
	pShowWindow.Call(mainWindow, SW_HIDE)
	releaseLayeredFrame()
	restoreOverlayInputLocked(restoreFocus)
	updateTimerIntervalLocked()
}

func redraw() {
	if mainWindow != 0 {
		pInvalidateRect.Call(mainWindow, 0, 1)
	}
}

func redrawNow() {
	if mainWindow == 0 {
		return
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		redraw()
		return
	}
	renderLayeredOverlay(hdc, mainWindow)
	pReleaseDC.Call(0, hdc)
}

func flushPendingOverlayShow() {
	if mainWindow == 0 {
		return
	}
	state.mu.Lock()
	showCmd := state.pendingOverlayShow
	mode := state.overlayMode
	state.pendingOverlayShow = 0
	state.mu.Unlock()
	if showCmd == 0 {
		return
	}
	redrawNow()
	if showCmd == SW_SHOW && mode == overlayMenu {
		// Modo menu normal: o overlay vira a janela ativa temporariamente para impedir
		// que a maioria dos jogos continue recebendo D-pad/X/O enquanto o usuário navega.
		pShowWindow.Call(mainWindow, uintptr(SW_SHOW))
		pSetWindowPos.Call(mainWindow, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
		forceOverlayForeground()
		state.mu.Lock()
		if state.overlayMode == overlayMenu && !state.gameDialogOpen {
			// Capture primeiro as mensagens do mouse. Depois prenda e mova o cursor,
			// impedindo que o jogo receba até mesmo o reposicionamento inicial.
			captureMouseInputLocked()
			clipMouseToOverlayLocked()
			ensureCursorHiddenLocked()
		}
		state.mu.Unlock()
		return
	}
	// Toast/bateria continuam sem ativar a janela. O menu sempre usa o caminho
	// foreground acima para não deixar o jogo captando comandos atrás.
	pShowWindow.Call(mainWindow, uintptr(SW_SHOWNOACTIVATE))
	pSetWindowPos.Call(mainWindow, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_SHOWWINDOW)
}

func hideCursorLocked() {
	ensureCursorHiddenLocked()
}

func ensureCursorHiddenLocked() {
	pSetCursor.Call(0)
	var info cursorInfo
	info.CbSize = uint32(unsafe.Sizeof(info))
	// Se a consulta falhar, só force uma nova chamada na primeira ocultação. Isso
	// evita acumular o contador sem evidência de que o cursor reapareceu.
	visible := !state.cursorHidden
	if ok, _, _ := pGetCursorInfo.Call(uintptr(unsafe.Pointer(&info))); ok != 0 {
		visible = info.Flags&CURSOR_SHOWING != 0
	}
	if !state.cursorHidden {
		state.cursorHidden = true
		state.cursorHideBalance = 0
	}
	if !visible {
		return
	}
	// Jogos e o próprio Windows podem elevar novamente o contador de exibição.
	// O total de chamadas feitas pelo DualCenter é preservado para ser desfeito
	// exatamente ao fechar o menu, sem deixar o cursor oculto fora do overlay.
	for state.cursorHideBalance < 64 {
		r, _, _ := pShowCursor.Call(0)
		state.cursorHideBalance++
		if int32(r) < 0 {
			break
		}
	}
	pSetCursor.Call(0)
}
func restoreCursorLocked() {
	// Solte a trava antes de devolver a posição para não prendê-la novamente no
	// ponto central do overlay.
	releaseMouseClipLocked()
	if state.cursorPositionSaved {
		pSetCursorPos.Call(
			uintptr(int64(state.savedCursorPosition.X)),
			uintptr(int64(state.savedCursorPosition.Y)),
		)
		state.cursorPositionSaved = false
	}
	if state.cursorHidden {
		// Desfaça somente as chamadas feitas pelo DualCenter. Jogos que tentem
		// tornar o cursor visível enquanto o menu está aberto não podem empurrar
		// nosso contador indefinidamente nem impedir a restauração ao fechar.
		for state.cursorHideBalance > 0 {
			pShowCursor.Call(1)
			state.cursorHideBalance--
		}
		state.cursorHidden = false
	}
}

func saveCursorPositionLocked() {
	if state.cursorPositionSaved {
		return
	}
	var position point
	if ok, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&position))); ok != 0 {
		state.savedCursorPosition = position
		state.cursorPositionSaved = true
	}
}

func clipMouseToOverlayLocked() {
	if mainWindow == 0 {
		return
	}
	saveCursorPositionLocked()
	var r rect
	if ok, _, _ := pGetWindowRect.Call(mainWindow, uintptr(unsafe.Pointer(&r))); ok != 0 {
		cx := r.Left + (r.Right-r.Left)/2
		cy := r.Top + (r.Bottom-r.Top)/2
		lock := rect{cx, cy, cx + 1, cy + 1}
		pClipCursor.Call(uintptr(unsafe.Pointer(&lock)))
		pSetCursorPos.Call(uintptr(int64(cx)), uintptr(int64(cy)))
	}
}

func releaseMouseClipLocked() {
	pClipCursor.Call(0)
}

func captureMouseInputLocked() {
	if mainWindow == 0 || state.overlayMode != overlayMenu || state.gameDialogOpen {
		return
	}
	if captured, _, _ := pGetCapture.Call(); captured != mainWindow {
		pSetCapture.Call(mainWindow)
	}
}

func releaseMouseInputLocked() {
	if captured, _, _ := pGetCapture.Call(); captured == mainWindow {
		pReleaseCapture.Call()
	}
}

func blockPhysicalInputLocked() {
	if state.physicalInputBlocked || state.overlayMode != overlayMenu || state.gameDialogOpen {
		return
	}
	if ok, _, err := pBlockInput.Call(1); ok != 0 {
		state.physicalInputBlocked = true
	} else {
		logf("BlockInput(TRUE) falhou; o mouse continuará protegido pelo foco, captura e ClipCursor: %v", err)
	}
}

func releasePhysicalInputLocked() {
	if !state.physicalInputBlocked {
		return
	}
	if ok, _, err := pBlockInput.Call(0); ok == 0 {
		logf("BlockInput(FALSE) não confirmou a liberação da entrada: %v", err)
	}
	// Se o Windows já liberou a entrada por CTRL+ALT+DEL, BlockInput(FALSE) pode
	// não confirmar a operação. Em ambos os casos o DualCenter deixou de possuir
	// o bloqueio e a próxima abertura deve tentar adquiri-lo normalmente.
	state.physicalInputBlocked = false
}

func restoreOverlayInputLocked(restoreFocus bool) {
	restoreCursorLocked()
	releaseMenuFocusLocked()
	restoreTargetInputLocked(restoreFocus)
	// Libere a entrada física por último. Assim nenhum movimento alcança o jogo
	// entre a restauração do cursor, o fechamento do overlay e a devolução do foco.
	releasePhysicalInputLocked()
}

func activateWindow(hwnd uintptr) {
	if hwnd == 0 || !validWindow(hwnd) {
		return
	}
	foreground, _, _ := pGetForegroundWindow.Call()
	currentThread, _, _ := pGetCurrentThreadId.Call()
	foregroundThread := uintptr(0)
	if foreground != 0 && validWindow(foreground) {
		foregroundThread, _, _ = pGetWindowThreadProcessId.Call(foreground, 0)
	}
	attached := false
	if foregroundThread != 0 && foregroundThread != currentThread {
		if ok, _, _ := pAttachThreadInput.Call(currentThread, foregroundThread, 1); ok != 0 {
			attached = true
		}
	}
	defer func() {
		if attached {
			pAttachThreadInput.Call(currentThread, foregroundThread, 0)
		}
	}()
	// Não use SW_RESTORE em toda janela ao sair do menu: em janelas maximizadas
	// ou jogos/telas cheias isso pode alterar o estado da janela que estava por
	// baixo do overlay. Só restaura se o Windows realmente marcou como minimizada.
	if minimized, _, _ := pIsIconic.Call(hwnd); minimized != 0 {
		pShowWindow.Call(hwnd, SW_RESTORE)
	}
	pBringWindowToTop.Call(hwnd)
	pSetForegroundWindow.Call(hwnd)
	pSetFocus.Call(hwnd)
}

func forceOverlayForeground() {
	activateWindow(mainWindow)
}

func captureMenuFocusLocked() {
	if mainWindow == 0 {
		return
	}
	style, _, _ := pGetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE)
	style &^= WS_EX_NOACTIVATE | WS_EX_TRANSPARENT
	style |= WS_EX_LAYERED | WS_EX_TOOLWINDOW | WS_EX_TOPMOST
	pSetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE, style)
	// Não mostre a janela aqui: ao abrir o menu logo após o toast de bateria, o
	// DWM podia exibir por um frame a superfície antiga do overlay compacto atrás
	// do menu. O fluxo correto é: ajustar estilo/posição, renderizar o frame novo
	// em flushPendingOverlayShow e só então mostrar a janela. Isso não adiciona
	// sleep/debounce e mantém os comandos imediatos.
	pSetWindowPos.Call(mainWindow, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_FRAMECHANGED)
}

func ensureMenuFocusLocked() {
	if state.overlayMode != overlayMenu || state.gameDialogOpen || mainWindow == 0 {
		return
	}
	foreground, _, _ := pGetForegroundWindow.Call()
	if foreground != mainWindow {
		activateWindow(mainWindow)
	}
	captureMouseInputLocked()
}

func restoreTargetInputLocked(restoreFocus bool) {
	releaseMouseInputLocked()
	if restoreFocus && validWindow(state.savedForeground) {
		activateWindow(state.savedForeground)
	}
	state.savedForeground = 0
}

func releaseMenuFocusLocked() {
	if mainWindow == 0 {
		return
	}
	style, _, _ := pGetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE)
	style |= WS_EX_NOACTIVATE | WS_EX_TRANSPARENT | WS_EX_LAYERED | WS_EX_TOOLWINDOW | WS_EX_TOPMOST
	pSetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE, style)
	pSetWindowPos.Call(mainWindow, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_FRAMECHANGED)
	releaseMouseInputLocked()
}

func validWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ok, _, _ := pIsWindow.Call(hwnd)
	return ok != 0
}

func windowFillsMonitor(hwnd uintptr) bool {
	if !validWindow(hwnd) {
		return false
	}
	var wr rect
	if ok, _, _ := pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); ok == 0 {
		return false
	}
	mr := monitorRectForWindow(hwnd)
	const tolerance int32 = 2
	return wr.Left <= mr.Left+tolerance &&
		wr.Top <= mr.Top+tolerance &&
		wr.Right >= mr.Right-tolerance &&
		wr.Bottom >= mr.Bottom-tolerance
}

func exclusiveFullscreenActive() bool {
	foreground, _, _ := pGetForegroundWindow.Call()
	if foreground == 0 || foreground == mainWindow || !windowFillsMonitor(foreground) {
		return false
	}
	var notificationState uint32
	hr, _, _ := pSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&notificationState)))
	return succeeded(hr) && notificationState == QUNS_RUNNING_D3D_FULL_SCREEN
}

func overlayTargetWindow() uintptr {
	if state.overlayMode == overlayMenu && validWindow(state.savedForeground) {
		return state.savedForeground
	}
	w, _, _ := pGetForegroundWindow.Call()
	if w != mainWindow && validWindow(w) {
		return w
	}
	if validWindow(state.savedForeground) {
		return state.savedForeground
	}
	return mainWindow
}

func monitorRectForWindow(hwnd uintptr) rect {
	if !validWindow(hwnd) {
		hwnd = mainWindow
	}
	mon, _, _ := pMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if mon != 0 {
		if r, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r != 0 {
			return mi.Monitor
		}
	}
	w, _, _ := pGetSystemMetrics.Call(0)
	h, _, _ := pGetSystemMetrics.Call(1)
	return rect{0, 0, int32(w), int32(h)}
}

func refreshOverlayLayoutLocked() {
	target := overlayTargetWindow()
	mr := monitorRectForWindow(target)
	if mr != state.menuMonitor {
		repositionCurrentOverlay(true)
		redraw()
	}
}

func repositionCurrentOverlay(show bool) {
	target := overlayTargetWindow()
	mr := monitorRectForWindow(target)
	mw := mr.Right - mr.Left
	mh := mr.Bottom - mr.Top
	if mw < 320 || mh < 240 {
		// Durante a reconexão de uma tela o Windows pode expor uma geometria
		// transitória. Não aplique tamanhos inválidos ao overlay.
		return
	}
	screenScale := math.Min(float64(mw)/1920.0, float64(mh)/1080.0)
	// Mantém a mesma proporção visual da referência em qualquer resolução.
	// 1920x1080 = 1x, 2560x1440 = 1,333x e 3840x2160 = 2x.
	if screenScale <= 0 || math.IsNaN(screenScale) || math.IsInf(screenScale, 0) {
		screenScale = 1
	}
	screenScale = math.Max(0.5, math.Min(screenScale, 4.0))
	state.menuMonitor = mr
	if math.Abs(state.displayScale-screenScale) > 0.0001 {
		clearScaleDependentArtworkCaches()
		state.displayScale = screenScale
	}
	var w, h, x, y int32
	switch state.overlayMode {
	case overlayMenu:
		// A v38 aprovada usava painéis cerca de 16,5% maiores e ancorados
		// próximo à parte inferior. A área visual permanece igual; somente a margem
		// transparente da janela cresce para acomodar a sombra sem recortes.
		state.scale = screenScale * 1.165
		baseW := int32(1620 * screenScale)
		baseH := int32(380 * screenScale)
		shadowPaddingX := menuShadowPaddingX()
		shadowPaddingY := menuShadowPaddingY()
		w = baseW + 2*shadowPaddingX
		h = baseH + 2*shadowPaddingY
		// Compense a margem extra para que as abas mantenham a posição aprovada.
		x = mr.Left + (mw-baseW)/2 - shadowPaddingX
		y = mr.Bottom - baseH - int32(60*screenScale) - shadowPaddingY
		if y < mr.Top+int32(20*screenScale) {
			y = mr.Top + int32(20*screenScale)
		}
	case overlayBattery:
		// Overlay compacto fiel ao mockup aprovado: cápsula de vidro pequena,
		// canto superior direito, controle branco, bateria azul e porcentagem.
		state.scale = screenScale
		w = int32(math.Round(184 * screenScale))
		h = int32(math.Round(64 * screenScale))
		if w < 168 {
			w = 168
		}
		if h < 56 {
			h = 56
		}
		marginX := int32(math.Round(40 * screenScale))
		marginY := int32(math.Round(40 * screenScale))
		if marginX < 22 {
			marginX = 22
		}
		if marginY < 22 {
			marginY = 22
		}
		x = mr.Right - w - marginX
		y = mr.Top + marginY
	default:
		// A notificação de modo jogo usa o tamanho 2 (médio), mas sua largura
		// é compacta e termina pouco depois da frase, sem área vazia excessiva.
		if strings.EqualFold(state.messageTitle, "MODO JOGO") {
			state.scale = screenScale * 0.72
			w = int32(400 * screenScale)
			h = int32(115 * screenScale)
		} else if strings.EqualFold(state.messageTitle, fullscreenUnavailableTitle) {
			// O aviso de fullscreen mantém apenas o respiro necessário ao redor da
			// linha mais larga. A área transparente externa continua reservada para a
			// sombra terminar suavemente.
			state.scale = screenScale
			shadowPadding := scaled(fullscreenWarningShadowPadding)
			titleWidth := measureTextWidth(state.messageTitle, scaled(21), FW_SEMIBOLD, "Segoe UI Variable Display")
			bodyWidth := measureTextWidth(state.messageBody, scaled(15), FW_MEDIUM, "Segoe UI Variable Text")
			contentWidth := titleWidth
			if bodyWidth > contentWidth {
				contentWidth = bodyWidth
			}
			if contentWidth > 0 {
				w = contentWidth + scaled(48) + 2*shadowPadding
			} else {
				w = int32(480*screenScale) + 2*shadowPadding
			}
			maximumWidth := mw - int32(math.Round(48*screenScale))
			if maximumWidth > 0 && w > maximumWidth {
				w = maximumWidth
			}
			h = int32(math.Round(92*screenScale)) + 2*shadowPadding
		} else {
			// Mensagens informativas mais longas mantêm espaço suficiente para
			// serem lidas sem cortar o conteúdo.
			state.scale = screenScale * 0.90
			w = int32(702 * screenScale)
			h = int32(144 * screenScale)
		}
		x = mr.Left + (mw-w)/2
		y = mr.Top + int32(math.Round(42*screenScale))
		if strings.EqualFold(state.messageTitle, fullscreenUnavailableTitle) {
			// Compensa a nova margem para manter o cartão na mesma posição visual.
			y -= scaled(fullscreenWarningShadowPadding)
		}
	}
	flags := uintptr(SWP_NOACTIVATE)
	if show {
		flags |= SWP_SHOWWINDOW
	}
	pSetWindowPos.Call(mainWindow, HWND_TOPMOST, uintptr(int64(x)), uintptr(int64(y)), uintptr(w), uintptr(h), flags)
}
