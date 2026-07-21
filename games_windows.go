// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func activateGameItemLocked(index int) {
	if index < 0 {
		return
	}
	if index >= len(state.games) {
		startGameFileDialogLocked()
		return
	}
	openGameActionMenuLocked(index)
}

func openGameActionMenuLocked(index int) {
	if index < 0 || index >= len(state.games) {
		return
	}
	state.gameMenuOpen = true
	state.gameMenuGame = index
	state.gameMenuSelection = 0
	state.confirmAction = ""
	redraw()
}

func gameMenuActionsLocked(index int) []string {
	if index < 0 || index >= len(state.games) {
		return nil
	}
	running := index < len(state.gameRunning) && state.gameRunning[index]
	if running {
		return []string{"Fechar jogo", "Remover jogo"}
	}
	return []string{"Abrir jogo", "Remover jogo"}
}

func handleGameMenuButtonsLocked(dpad, prevDpad byte, cross, prevCross, circle, prevCircle bool) {
	if !state.gameMenuOpen {
		return
	}
	if circle && !prevCircle {
		state.gameMenuOpen = false
		state.gameMenuGame = -1
		state.gameMenuSelection = 0
		state.confirmAction = ""
		redraw()
		return
	}
	actions := gameMenuActionsLocked(state.gameMenuGame)
	if len(actions) == 0 {
		state.gameMenuOpen = false
		state.gameMenuGame = -1
		state.gameMenuSelection = 0
		state.confirmAction = ""
		redraw()
		return
	}
	state.gameMenuSelection = clamp(state.gameMenuSelection, 0, len(actions)-1)
	if dpad != prevDpad {
		switch dpad {
		case 0, 7, 1: // cima e diagonais superiores
			if state.gameMenuSelection > 0 {
				state.gameMenuSelection--
			}
			state.confirmAction = ""
		case 4, 5, 3: // baixo e diagonais inferiores
			if state.gameMenuSelection+1 < len(actions) {
				state.gameMenuSelection++
			}
			state.confirmAction = ""
		}
		redraw()
	}
	if cross && !prevCross {
		executeGameMenuActionLocked(actions[state.gameMenuSelection])
	}
}

func executeGameMenuActionLocked(action string) {
	index := state.gameMenuGame
	if index < 0 || index >= len(state.games) {
		state.gameMenuOpen = false
		state.gameMenuGame = -1
		state.gameMenuSelection = 0
		state.confirmAction = ""
		redraw()
		return
	}
	game := state.games[index]
	switch action {
	case "Abrir jogo":
		state.gameMenuOpen = false
		state.gameMenuGame = -1
		state.gameMenuSelection = 0
		state.confirmAction = ""
		finalizeHideOverlayLocked()
		go launchGameExecutable(game.Path)
	case "Fechar jogo":
		key := fmt.Sprintf("closegame:%d", index)
		now := time.Now()
		if state.confirmAction != key || !now.Before(state.confirmUntil) {
			state.confirmAction = key
			state.confirmUntil = now.Add(4 * time.Second)
			redraw()
			return
		}
		state.gameMenuOpen = false
		state.gameMenuGame = -1
		state.gameMenuSelection = 0
		state.confirmAction = ""
		finalizeHideOverlayLocked()
		go closeGameExecutable(game.Path)
	case "Remover jogo":
		key := fmt.Sprintf("removegame:%d", index)
		now := time.Now()
		if state.confirmAction != key || !now.Before(state.confirmUntil) {
			state.confirmAction = key
			state.confirmUntil = now.Add(4 * time.Second)
			redraw()
			return
		}
		removeGameLocked(index)
	}
}

func removeGameLocked(index int) {
	if index < 0 || index >= len(state.games) {
		return
	}
	key := strings.ToLower(filepath.Clean(state.games[index].Path))
	if iconHandle, ok := gameIconCache[key]; ok {
		if iconHandle != 0 {
			pDestroyIcon.Call(iconHandle)
		}
		delete(gameIconCache, key)
	}
	state.games = append(state.games[:index], state.games[index+1:]...)
	if index < len(state.gameRunning) {
		state.gameRunning = append(state.gameRunning[:index], state.gameRunning[index+1:]...)
	}
	state.gameMenuOpen = false
	state.gameMenuGame = -1
	state.gameMenuSelection = 0
	state.confirmAction = ""
	total := len(state.games) + 1
	if state.row[tabGames] >= total {
		state.row[tabGames] = total - 1
	}
	if state.row[tabGames] < 0 {
		state.row[tabGames] = 0
	}
	saveSettingsLocked()
	redraw()
}

func startGameFileDialogLocked() {
	if state.gameDialogOpen {
		return
	}
	pendingGameDialogMu.Lock()
	pendingGameDialogPath = ""
	pendingGameDialogDevice = state.menuDevice
	pendingGameDialogMu.Unlock()
	// Preserve o overlay como janela ativa até o seletor assumir o foco. A versão
	// anterior escondia o menu e reativava o jogo antes de postar a abertura do
	// diálogo; nesse pequeno intervalo, o mesmo X usado no mosaico "+" também era
	// recebido pelo jogo. O cursor é liberado para permitir o uso normal do mouse
	// no seletor, mas o foreground não volta ao jogo.
	state.gameDialogOpen = true
	state.confirmAction = ""
	releaseMouseInputLocked()
	restoreCursorLocked()
	releasePhysicalInputLocked()
	// O diálogo é aberto na mesma thread de interface do DualCenter. Dessa forma,
	// o Windows reconhece a ação direta do usuário, mantém o seletor em primeiro
	// plano e o associa ao aplicativo em vez de deixá-lo atrás do jogo.
	posted, _, postErr := pPostMessageW.Call(mainWindow, WM_APP_OPEN_GAME_DIALOG, 0, 0)
	if posted == 0 {
		state.gameDialogOpen = false
		blockPhysicalInputLocked()
		hideCursorLocked()
		clipMouseToOverlayLocked()
		logf("não foi possível abrir o seletor de jogo: %v", postErr)
	}
}

func openPendingGameDialog() {
	path := chooseGameExecutable()
	pendingGameDialogMu.Lock()
	pendingGameDialogPath = path
	pendingGameDialogMu.Unlock()
	handlePendingGameDialog()
}

func utf16StringFromPointer(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	values := make([]uint16, 0, 260)
	for i := uintptr(0); i < 32768; i++ {
		v := *(*uint16)(unsafe.Add(ptr, i*2))
		if v == 0 {
			break
		}
		values = append(values, v)
	}
	return syscall.UTF16ToString(values)
}

func chooseGameExecutable() string {
	// A thread principal já utiliza COM, mas CoInitializeEx é chamado novamente
	// para manter esta rotina segura caso seja reutilizada isoladamente.
	hr, _, _ := pCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	initialized := succeeded(hr)
	if initialized {
		defer pCoUninitialize.Call()
	}

	var dialog unsafe.Pointer
	hr, _, _ = pCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		CLSCTX_ALL,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if !succeeded(hr) || dialog == nil {
		logf("IFileOpenDialog indisponível: HRESULT=%#x", uint32(hr))
		return ""
	}
	defer vcall(dialog, 2) // IUnknown::Release

	options := uintptr(FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST | FOS_FILEMUSTEXIST | FOS_DONTADDTORECENT)
	vcall(dialog, 9, options) // IFileDialog::SetOptions

	filters := [1]comdlgFilterSpec{{Name: utf16Ptr("Executáveis (*.exe)"), Spec: utf16Ptr("*.exe")}}
	vcall(dialog, 4, 1, uintptr(unsafe.Pointer(&filters[0]))) // SetFileTypes
	vcall(dialog, 5, 1)                                       // SetFileTypeIndex
	vcall(dialog, 17, uintptr(unsafe.Pointer(utf16Ptr("Selecionar jogo — DualCenter"))))
	vcall(dialog, 22, uintptr(unsafe.Pointer(utf16Ptr("exe"))))

	// Como o diálogo pertence ao DualCenter e é aberto pela thread que recebeu o
	// comando do controle, ele aparece acima do jogo sem exigir Alt+Tab.
	hr = vcall(dialog, 3, mainWindow) // IModalWindow::Show
	if !succeeded(hr) {
		return "" // cancelamento pelo usuário também chega aqui
	}

	var item unsafe.Pointer
	hr = vcall(dialog, 20, uintptr(unsafe.Pointer(&item))) // IFileDialog::GetResult
	if !succeeded(hr) || item == nil {
		return ""
	}
	defer vcall(item, 2)

	var pathPtr unsafe.Pointer
	hr = vcall(item, 5, SIGDN_FILESYSPATH, uintptr(unsafe.Pointer(&pathPtr))) // IShellItem::GetDisplayName
	if !succeeded(hr) || pathPtr == nil {
		return ""
	}
	defer pCoTaskMemFree.Call(uintptr(pathPtr))
	value := strings.TrimSpace(utf16StringFromPointer(pathPtr))
	if value == "" || !strings.EqualFold(filepath.Ext(value), ".exe") {
		return ""
	}
	return filepath.Clean(value)
}

func handlePendingGameDialog() {
	pendingGameDialogMu.Lock()
	path := pendingGameDialogPath
	device := pendingGameDialogDevice
	pendingGameDialogPath = ""
	pendingGameDialogDevice = 0
	pendingGameDialogMu.Unlock()

	state.mu.Lock()
	state.gameDialogOpen = false
	if path != "" && strings.EqualFold(filepath.Ext(path), ".exe") {
		duplicate := -1
		for i := range state.games {
			if strings.EqualFold(filepath.Clean(state.games[i].Path), filepath.Clean(path)) {
				duplicate = i
				break
			}
		}
		if duplicate < 0 {
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			state.games = append(state.games, gameEntry{Name: name, Path: path})
			state.gameRunning = append(state.gameRunning, false)
			state.row[tabGames] = len(state.games) - 1
		} else {
			state.row[tabGames] = duplicate
		}
		saveSettingsLocked()
	}
	state.menuPanel = tabGames
	state.gamesFocused = true
	state.menuPos = tabPositionLocked(tabGames)
	ensureSelectedTabVisibleLocked()
	if device != 0 && state.overlayMode == overlayMenu {
		// O menu permaneceu aberto como proprietário do seletor. Apenas retomamos
		// o modo controle, sem recriar o menu nem devolver o foco ao jogo.
		state.menuDevice = device
		state.lastMenuFocusCheck = time.Time{}
		blockPhysicalInputLocked()
		hideCursorLocked()
		captureMenuFocusLocked()
		clipMouseToOverlayLocked()
		state.pendingOverlayShow = SW_SHOW
		updateTimerIntervalLocked()
	}
	state.mu.Unlock()
	flushPendingOverlayShow()
}

func launchGameExecutable(path string) {
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		logf("falha ao iniciar jogo %q: %v", path, err)
		return
	}
	// Esta função já é iniciada em uma goroutine; aguardar aqui libera os recursos
	// do processo sem criar uma segunda goroutine para o mesmo jogo.
	if err := cmd.Wait(); err != nil {
		logf("jogo encerrado com erro %q: %v", path, err)
	}
}

func closeGameExecutable(path string) {
	pids := runningPIDsForPath(path)
	for _, pid := range pids {
		// Primeiro tenta o fechamento direto e forçado pelo Windows. Alguns jogos
		// antigos ignoram o encerramento normal, então usamos /F para garantir.
		cmd := exec.Command("taskkill.exe", "/PID", fmt.Sprintf("%d", pid), "/T", "/F")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		if err := cmd.Run(); err != nil {
			logf("taskkill forçado falhou ao fechar jogo %q pid=%d: %v", path, pid, err)
			forceTerminateProcess(pid)
		}
	}
}

func forceTerminateProcess(pid uint32) {
	ph, _, _ := pOpenProcess.Call(PROCESS_TERMINATE, 0, uintptr(pid))
	if ph == 0 {
		return
	}
	defer pCloseHandle.Call(ph)
	pTerminateProcess.Call(ph, 1)
}

func runningPIDsForPath(path string) []uint32 {
	target := strings.ToLower(filepath.Clean(path))
	h, _, _ := pCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if h == 0 || h == INVALID_HANDLE_VALUE {
		return nil
	}
	defer pCloseHandle.Call(h)
	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	buf := make([]uint16, 32768)
	r, _, _ := pProcess32FirstW.Call(h, uintptr(unsafe.Pointer(&entry)))
	var out []uint32
	for r != 0 {
		if entry.ProcessID != 0 {
			ph, _, _ := pOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(entry.ProcessID))
			if ph != 0 {
				n := uint32(len(buf))
				ok, _, _ := pQueryFullProcessImageNameW.Call(ph, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
				pCloseHandle.Call(ph)
				if ok != 0 && n > 0 {
					candidate := strings.ToLower(filepath.Clean(syscall.UTF16ToString(buf[:n])))
					if candidate == target {
						out = append(out, entry.ProcessID)
					}
				}
			}
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		r, _, _ = pProcess32NextW.Call(h, uintptr(unsafe.Pointer(&entry)))
	}
	return out
}

func tabVisibleLocked(tab int) bool {
	for slot := 0; slot < 5; slot++ {
		pos := state.visibleStart + slot
		if pos >= 0 && pos < len(state.tabOrder) && state.tabOrder[pos] == tab {
			return true
		}
	}
	return false
}

func runningExecutableSet() map[string]bool {
	set := map[string]bool{}
	h, _, _ := pCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if h == 0 || h == INVALID_HANDLE_VALUE {
		return set
	}
	defer pCloseHandle.Call(h)
	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	buf := make([]uint16, 32768)
	r, _, _ := pProcess32FirstW.Call(h, uintptr(unsafe.Pointer(&entry)))
	for r != 0 {
		if entry.ProcessID != 0 {
			ph, _, _ := pOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(entry.ProcessID))
			if ph != 0 {
				n := uint32(len(buf))
				ok, _, _ := pQueryFullProcessImageNameW.Call(ph, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
				pCloseHandle.Call(ph)
				if ok != 0 && n > 0 {
					path := strings.ToLower(filepath.Clean(syscall.UTF16ToString(buf[:n])))
					set[path] = true
				}
			}
		}
		entry.Size = uint32(unsafe.Sizeof(processEntry32{}))
		r, _, _ = pProcess32NextW.Call(h, uintptr(unsafe.Pointer(&entry)))
	}
	return set
}

func refreshGameStatesLocked(now time.Time) {
	if state.overlayMode != overlayMenu || len(state.games) == 0 || !tabVisibleLocked(tabGames) || state.gamePollInFlight {
		return
	}
	if !state.lastGamePoll.IsZero() && now.Sub(state.lastGamePoll) < 2*time.Second {
		return
	}
	state.lastGamePoll = now
	paths := make([]string, len(state.games))
	for i := range state.games {
		paths[i] = strings.ToLower(filepath.Clean(state.games[i].Path))
	}
	state.gamePollInFlight = true
	go refreshGameStateSnapshot(paths)
}

func applyGameStateSnapshotLocked(paths []string, runningSet map[string]bool) (applied, changed bool) {
	if len(paths) != len(state.games) {
		return false, false
	}
	for i := range state.games {
		if paths[i] != strings.ToLower(filepath.Clean(state.games[i].Path)) {
			return false, false
		}
	}
	if len(state.gameRunning) != len(state.games) {
		state.gameRunning = make([]bool, len(state.games))
		changed = true
	}
	for i, path := range paths {
		running := runningSet[path]
		if state.gameRunning[i] != running {
			state.gameRunning[i] = running
			changed = true
		}
	}
	return true, changed
}

func refreshGameStateSnapshot(paths []string) {
	runningSet := runningExecutableSet()
	state.mu.Lock()
	state.gamePollInFlight = false
	applied, changed := applyGameStateSnapshotLocked(paths, runningSet)
	if !applied {
		// A biblioteca mudou durante a varredura. Permita uma atualização imediata
		// com a nova lista em vez de publicar um resultado obsoleto.
		state.lastGamePoll = time.Time{}
	}
	state.mu.Unlock()
	if changed {
		redraw()
	}
}
