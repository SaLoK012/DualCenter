// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"dualcenter/internal/version"
	"fmt"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	runtime.LockOSThread()
	initPaths()
	initLog()
	loadSettings()
	if handleMaintenanceCommand() {
		return
	}
	defer func() {
		state.mu.Lock()
		restoreOverlayInputLocked(false)
		state.mu.Unlock()
		shutdownAudio()
		if r := recover(); r != nil {
			logf("PANIC: %v\r\n%s", r, debug.Stack())
		}
	}()
	if !acquireSingleInstance() {
		return
	}
	defer func() {
		if mutexHandle != 0 {
			pCloseHandle.Call(mutexHandle)
			mutexHandle = 0
		}
	}()
	enableDPIAwareness()
	initControllerArtwork()
	defer shutdownControllerArtwork()
	if hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(utf16Ptr("DualSenseHUBOverlayWindow"))), 0); hwnd != 0 {
		showFatal("DualSenseHUB aberto", "Feche o DualSenseHUB antes de executar o DualCenter.")
		return
	}
	hInstance, _, _ := pGetModuleHandleW.Call(0)
	appIcon, _, _ := pLoadIconW.Call(hInstance, 1)
	if appIcon == 0 {
		appIcon, _, _ = pLoadIconW.Call(0, 32512) // IDI_APPLICATION
	}
	className := utf16Ptr(windowClassName)
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HIcon:         appIcon,
		LpszClassName: className,
		HIconSm:       appIcon,
	}
	if r, _, e := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		showFatal("Falha ao iniciar", fmt.Sprintf("RegisterClassExW: %v", e))
		return
	}
	exStyle := uintptr(WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_NOACTIVATE)
	mainWindow, _, _ = pCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16Ptr(appName))), WS_POPUP, 0, 0, 600, 120, 0, 0, hInstance, 0)
	if mainWindow == 0 {
		showFatal("Falha ao iniciar", "Não foi possível criar a janela do overlay.")
		return
	}
	initializeGameBarChoice()
	if settingsRecovered {
		showFatal("Configurações restauradas", "O arquivo de configurações estava inválido e foi preservado com a extensão .corrupt. O DualCenter iniciou com valores seguros.")
	}
	pSendMessageW.Call(mainWindow, WM_SETICON, 1, appIcon)
	pSendMessageW.Call(mainWindow, WM_SETICON, 0, appIcon)
	rawInputRegistered = registerControllerRawInput(mainWindow)
	if !rawInputRegistered {
		logf("registro Raw Input inicial falhou; novas tentativas serão feitas automaticamente")
	}
	if rawInputRegistered {
		setTimerInterval(0)
	} else {
		setTimerInterval(500)
	}
	initAudioEndpoint()
	state.defaultAudioDeviceID = defaultAudioID()
	state.audioName = currentAudioNameForID(state.defaultAudioDeviceID)
	state.startupEnabled = isStartupEnabled()
	state.mu.Lock()
	if err := applyTaskbarIconStyleLocked(); err != nil {
		logf("falha ao aplicar preferência do ícone na barra de tarefas: %v", err)
	}
	saveSettingsLocked()
	state.mu.Unlock()
	logf("inicialização concluída: versão=%s go=%s", version.Display(), runtime.Version())
	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func showFatal(t, b string) {
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(b))), uintptr(unsafe.Pointer(utf16Ptr(appName+" — "+t))), 0x30)
}

func acquireSingleInstance() bool {
	h, _, e := pCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(utf16Ptr(mutexName))))
	if h == 0 {
		return false
	}
	mutexHandle = h
	if errno, ok := e.(syscall.Errno); ok && uint32(errno) == ERROR_ALREADY_EXISTS {
		pCloseHandle.Call(h)
		mutexHandle = 0
		return false
	}
	return true
}

func enableDPIAwareness() {
	syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext").Call(^uintptr(3))
}

func registerControllerRawInput(hwnd uintptr) bool {
	d := []rawInputDevice{{1, 5, RIDEV_INPUTSINK | RIDEV_DEVNOTIFY, hwnd}, {1, 4, RIDEV_INPUTSINK | RIDEV_DEVNOTIFY, hwnd}}
	r, _, e := pRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&d[0])), uintptr(len(d)), unsafe.Sizeof(d[0]))
	if r == 0 {
		logf("RegisterRawInputDevices falhou: %v", e)
		return false
	}
	return true
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) (result uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logf("PANIC wndProc msg=%#x: %v\r\n%s", message, r, debug.Stack())
			result = 0
		}
	}()
	if taskbarCreatedMessage != 0 && message == taskbarCreatedMessage {
		handleTaskbarCreated()
		return 0
	}
	switch message {
	case WM_APP_EXIT:
		state.mu.Lock()
		completeCircleReleaseLocked(false)
		_ = removeTaskbarIconLocked()
		restoreOverlayInputLocked(false)
		state.mu.Unlock()
		pPostQuitMessage.Call(0)
		return 0
	case WM_APP_TASKBAR_ICON:
		handleTaskbarIconEvent(lParam)
		return 0
	case WM_APP_OPEN_GAME_DIALOG:
		openPendingGameDialog()
		return 0
	case WM_APP_ADD_GAME:
		handlePendingGameDialog()
		return 0
	case WM_INPUT:
		processRawInput(lParam)
		return 0
	case WM_INPUT_DEVICE_CHANGE:
		forgetRawInputDevice(lParam)
		state.mu.Lock()
		delete(state.btRequest, lParam)
		delete(state.btAttempts, lParam)
		if wParam == GIDC_REMOVAL {
			delete(state.controllers, lParam)
			if state.circleReleasePending && state.circleReleaseDevice == lParam {
				completeCircleReleaseLocked(true)
			}
			if state.menuDevice == lParam && state.overlayMode == overlayMenu {
				closeMenuLocked()
			}
			if state.activeDevice == lParam {
				selectAnyControllerLocked()
			}
		}
		state.mu.Unlock()
		redraw()
		return 0
	case WM_TIMER:
		if wParam == DISPLAY_RELAYOUT_TIMER_ID {
			pKillTimer.Call(hwnd, DISPLAY_RELAYOUT_TIMER_ID)
			displayRelayoutArmed = false
			handleDisplayRelayout()
			return 0
		}
		onTimer()
		return 0
	case WM_PAINT:
		paint(hwnd)
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_SETCURSOR:
		if state.cursorHidden {
			pSetCursor.Call(0)
			return 1
		}
	case WM_MOUSEACTIVATE:
		return MA_NOACTIVATE
	case WM_DISPLAYCHANGE, WM_DPICHANGED, WM_SETTINGCHANGE:
		// SetWindowPos pode disparar WM_DPICHANGED de forma síncrona. Fazer o
		// reposicionamento enquanto state.mu está bloqueado causa reentrada no
		// wndProc e congela o processo ao alternar entre monitor e TV. Aguardar
		// brevemente também evita usar dimensões transitórias durante a troca.
		scheduleDisplayRelayout(hwnd)
		return 0
	case WM_CLOSE:
		// Algumas mudanças de topologia podem produzir um WM_CLOSE transitório.
		// Nesse pequeno intervalo, apenas recolha o overlay e mantenha o serviço
		// residente. Fora da troca de tela, preserve o encerramento normal.
		if time.Now().Before(displayTransitionUntil) {
			state.mu.Lock()
			if state.overlayMode != overlayHidden {
				finalizeHideOverlayLocked()
			}
			state.mu.Unlock()
			logf("WM_CLOSE ignorado durante transição de tela")
			return 0
		}
		state.mu.Lock()
		completeCircleReleaseLocked(false)
		_ = removeTaskbarIconLocked()
		restoreOverlayInputLocked(false)
		state.mu.Unlock()
		pPostQuitMessage.Call(0)
		return 0
	case WM_DESTROY:
		state.mu.Lock()
		completeCircleReleaseLocked(false)
		_ = removeTaskbarIconLocked()
		restoreOverlayInputLocked(false)
		state.mu.Unlock()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func scheduleDisplayRelayout(hwnd uintptr) {
	// Reinicia um timer curto para agrupar a sequência de mensagens enviada pelo
	// Windows ao ligar/desligar uma TV, trocar o monitor principal ou mudar DPI.
	displayTransitionUntil = time.Now().Add(2 * time.Second)
	if displayRelayoutArmed {
		pKillTimer.Call(hwnd, DISPLAY_RELAYOUT_TIMER_ID)
	}
	pSetTimer.Call(hwnd, DISPLAY_RELAYOUT_TIMER_ID, 250, 0)
	displayRelayoutArmed = true
}

func handleDisplayRelayout() {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.overlayMode == overlayHidden {
		state.menuMonitor = rect{}
		return
	}
	state.menuMonitor = rect{}
	repositionCurrentOverlay(true)
	redraw()
}
