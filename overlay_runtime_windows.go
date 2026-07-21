// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func requestPowerActionLocked(action string) {
	now := time.Now()
	if state.confirmAction == action && now.Before(state.confirmUntil) {
		state.confirmAction = ""
		seconds := state.warningSeconds
		// Responde imediatamente ao segundo X e executa o comando fora da thread da interface.
		finalizeHideOverlayLocked()
		if action == "suspend" {
			go executeSuspendAction()
		} else {
			go executePowerAction(action, seconds)
		}
		return
	}
	state.confirmAction = action
	state.confirmUntil = now.Add(4 * time.Second)
	redraw()
}

func executePowerAction(action string, seconds int) {
	arg := "/s"
	label := "desligar"
	if action == "restart" {
		arg = "/r"
		label = "reiniciar"
	}
	cmd := exec.Command("shutdown.exe", arg, "/t", fmt.Sprint(seconds), "/c", fmt.Sprintf("Seu computador irá %s em %d segundos", label, seconds))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	if err := cmd.Run(); err != nil {
		logf("falha ao solicitar %s do Windows: %v", action, err)
	}
}

func cancelPendingShutdown() {
	cmd := exec.Command("shutdown.exe", "/a")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	if err := cmd.Start(); err != nil {
		showMessageLocked("MODO JOGO", "Não foi possível cancelar", 2600*time.Millisecond)
		return
	}
	// O Windows remove o aviso nativo; apenas liberamos o processo em segundo plano.
	go func() {
		if err := cmd.Wait(); err != nil {
			logf("não foi possível cancelar o desligamento: %v", err)
		}
	}()
}

func handleAudioPickerLocked(dpad, prevDpad byte, cross, prevCross, circle, prevCircle bool) {
	if circle && !prevCircle {
		state.audioPicker = false
		redraw()
		return
	}
	if len(state.audioOutputs) == 0 {
		return
	}
	// A lista pode mudar se uma saída for desconectada enquanto o seletor está
	// aberto. Normalize o índice antes de navegar ou confirmar para evitar panic.
	state.audioOutputIndex = clamp(state.audioOutputIndex, 0, len(state.audioOutputs)-1)
	if dpad != prevDpad {
		if dpad == 0 {
			state.audioOutputIndex = (state.audioOutputIndex + len(state.audioOutputs) - 1) % len(state.audioOutputs)
		} else if dpad == 4 {
			state.audioOutputIndex = (state.audioOutputIndex + 1) % len(state.audioOutputs)
		}
		redraw()
	}
	if cross && !prevCross {
		selected := state.audioOutputs[state.audioOutputIndex]
		if setDefaultAudioEndpoint(selected.ID) {
			bindDefaultAudioEndpoint()
			state.defaultAudioDeviceID = selected.ID
			state.audioName = selected.Name
			state.lastAudioPoll = time.Now()
			state.audioPicker = false
		} else {
			logf("falha ao alterar saída de áudio para %q", selected.Name)
		}
		redraw()
	}
}

func currentMenuSelectionKeyLocked() menuSelectionKey {
	row := -1
	if state.menuPanel >= 0 && state.menuPanel < len(state.row) {
		row = state.row[state.menuPanel]
	}
	return menuSelectionKey{
		panel:             state.menuPanel,
		position:          state.menuPos,
		row:               row,
		gameMenuSelection: state.gameMenuSelection,
		organizing:        state.organizing,
		gamesFocused:      state.gamesFocused,
		gameMenuOpen:      state.gameMenuOpen,
		audioPicker:       state.audioPicker,
		volumeAdjust:      state.volumeAdjust,
	}
}

func selectionFrameAtLocked(now time.Time) (int, bool) {
	if state.selectionStartedAt.IsZero() {
		return selectionTransitionFrames, false
	}
	elapsed := now.Sub(state.selectionStartedAt)
	if elapsed >= selectionTransitionDuration {
		return selectionTransitionFrames, false
	}
	if elapsed <= 0 {
		return 0, true
	}
	progress := float64(elapsed) / float64(selectionTransitionDuration)
	// Ease-out cúbico: resposta imediata no controle e assentamento suave no fim.
	eased := 1 - math.Pow(1-progress, 3)
	frame := int(math.Round(eased * selectionTransitionFrames))
	return clamp(frame, 0, selectionTransitionFrames), true
}

func refreshSelectionTransitionLocked(_ time.Time) {
	key := currentMenuSelectionKeyLocked()
	if !state.selectionKeyValid || key != state.selectionKey {
		state.selectionKey = key
		state.selectionKeyValid = true
	}
	// A resposta visual acompanha o comando no primeiro frame. A antiga transição
	// de 120 ms obrigava a recompor o overlay em 60 Hz e fazia a navegação parecer
	// atrasada, principalmente quando a superfície tinha dimensões de 4K.
	state.selectionStartedAt = time.Time{}
	state.selectionFrame = selectionTransitionFrames
}

func advanceSelectionTransitionLocked(_ time.Time) bool {
	return false
}

func selectionTransitionStrengthLocked() float64 {
	progress := float64(clamp(state.selectionFrame, 0, selectionTransitionFrames)) / selectionTransitionFrames
	return 0.72 + 0.28*progress
}

func setTimerInterval(ms uint32) {
	if mainWindow == 0 {
		return
	}
	if ms == 0 {
		if timerInterval != 0 {
			pKillTimer.Call(mainWindow, 1)
			timerInterval = 0
		}
		return
	}
	if ms < 16 {
		ms = 16
	}
	if timerInterval == ms {
		return
	}
	pSetTimer.Call(mainWindow, 1, uintptr(ms), 0)
	timerInterval = ms
}

func updateTimerIntervalLocked() {
	interval := uint32(0)
	if !rawInputRegistered {
		interval = 500
	}
	if state.circleReleasePending {
		interval = 50
	} else if state.overlayMode == overlayMenu {
		// A navegação chega por Raw Input e invalida a janela imediatamente. O timer
		// de 10 Hz fica apenas para manutenção de foco, áudio e estado dos jogos.
		interval = 100
	} else if state.overlayMode == overlayBattery || state.overlayMode == overlayMessage {
		interval = 100
	} else {
		for _, c := range state.controllers {
			if c.psDown || !c.lastPSPress.IsZero() {
				interval = 50
				break
			}
		}
	}
	setTimerInterval(interval)
}

func currentAudioNameForID(id string) string {
	outputs := enumerateAudioOutputs()
	for i := range outputs {
		if strings.EqualFold(outputs[i].ID, id) {
			return outputs[i].Name
		}
	}
	if len(outputs) > 0 {
		return outputs[currentAudioOutputIndex(outputs)].Name
	}
	return "Padrão"
}

func refreshAudioStateLocked(force bool) bool {
	id := defaultAudioID()
	if !force && strings.EqualFold(id, state.defaultAudioDeviceID) {
		return false
	}
	oldID := state.defaultAudioDeviceID
	oldName := state.audioName
	state.defaultAudioDeviceID = id
	state.audioName = currentAudioNameForID(id)
	if !strings.EqualFold(oldID, id) {
		bindDefaultAudioEndpoint()
	}
	if state.audioPicker {
		state.audioOutputs = enumerateAudioOutputs()
		state.audioOutputIndex = currentAudioOutputIndex(state.audioOutputs)
	}
	return !strings.EqualFold(oldID, state.defaultAudioDeviceID) || oldName != state.audioName
}

func pollAudioStateLocked(now time.Time) {
	if state.overlayMode != overlayMenu {
		return
	}
	interval := time.Second
	if !state.lastAudioPoll.IsZero() && now.Sub(state.lastAudioPoll) < interval {
		return
	}
	state.lastAudioPoll = now
	if refreshAudioStateLocked(false) {
		redraw()
	}
}

func onTimer() {
	state.mu.Lock()
	now := time.Now()
	if !rawInputRegistered && (lastRawInputRetry.IsZero() || now.Sub(lastRawInputRetry) >= 2*time.Second) {
		lastRawInputRetry = now
		rawInputRegistered = registerControllerRawInput(mainWindow)
		if rawInputRegistered {
			logf("Raw Input registrado com sucesso após nova tentativa")
		}
	}
	for device, c := range state.controllers {
		if c.psDown && now.Sub(c.lastReportAt) > 900*time.Millisecond {
			c.psDown = false
		}
		if c.psDown && !c.longTriggered && now.Sub(c.psDownAt) >= 3*time.Second {
			c.longTriggered = true
			c.lastPSPress = time.Time{}
			selectControllerLocked(device, c)
			if state.blockShutdown {
				// Bloqueio mantido silencioso para não exibir overlay extra de desligamento.
			} else {
				finalizeHideOverlayLocked()
				go executePowerAction("shutdown", state.warningSeconds)
			}
			break
		}
	}
	for _, c := range state.controllers {
		if !c.lastPSPress.IsZero() && now.Sub(c.lastPSPress) > psDoublePressWindow {
			c.lastPSPress = time.Time{}
		}
	}
	if (state.overlayMode == overlayBattery || state.overlayMode == overlayMessage) && !state.toastHideAt.IsZero() && now.After(state.toastHideAt) {
		finalizeHideOverlayLocked()
	}
	if state.overlayMode != overlayHidden && (state.lastLayoutCheck.IsZero() || now.Sub(state.lastLayoutCheck) >= 250*time.Millisecond) {
		state.lastLayoutCheck = now
		refreshOverlayLayoutLocked()
	}
	if state.overlayMode == overlayMenu && !state.gameDialogOpen {
		// Alguns jogos tornam o cursor visível novamente a cada frame. Confirme o
		// estado enquanto o menu estiver aberto e refaça a ocultação somente quando
		// o Windows informar que a seta reapareceu.
		ensureCursorHiddenLocked()
		if advanceSelectionTransitionLocked(now) {
			redraw()
		}
		if state.lastMenuFocusCheck.IsZero() || now.Sub(state.lastMenuFocusCheck) >= 150*time.Millisecond {
			state.lastMenuFocusCheck = now
			ensureMenuFocusLocked()
		}
	}
	pollAudioStateLocked(now)
	refreshGameStatesLocked(now)
	updateTimerIntervalLocked()
	state.mu.Unlock()
	flushPendingOverlayShow()
}
