// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"time"
)

func handleMenuButtonsLocked(dpad, prevDpad byte, cross, prevCross, circle, prevCircle, triangle, prevTriangle bool) {
	if state.audioPicker {
		handleAudioPickerLocked(dpad, prevDpad, cross, prevCross, circle, prevCircle)
		return
	}
	if state.volumeAdjust {
		if dpad != prevDpad {
			if dpad == 0 || dpad == 2 || dpad == 1 || dpad == 7 {
				volumeStep(0.02)
			} else if dpad == 4 || dpad == 6 || dpad == 3 || dpad == 5 {
				volumeStep(-0.02)
			}
		}
		if circle && !prevCircle {
			state.volumeAdjust = false
		}
		redraw()
		return
	}

	if state.gameMenuOpen {
		handleGameMenuButtonsLocked(dpad, prevDpad, cross, prevCross, circle, prevCircle)
		return
	}

	if state.organizing {
		if circle && !prevCircle {
			finishOrganizingLocked(false)
			redraw()
			return
		}
		if (triangle && !prevTriangle) || (cross && !prevCross) {
			finishOrganizingLocked(true)
			redraw()
			return
		}
		if dpad != prevDpad {
			if dpad == 6 || dpad == 5 || dpad == 7 {
				moveOrganizedTabLocked(-1)
			}
			if dpad == 2 || dpad == 1 || dpad == 3 {
				moveOrganizedTabLocked(1)
			}
			redraw()
		}
		return
	}

	// A aba Jogos possui dois níveis de foco. Fora da grade, esquerda/direita
	// continuam trocando as abas. Dentro da grade, X e Triângulo abrem o menu
	// contextual compacto do jogo. Abrir, fechar e remover são executados
	// somente por esse menu; o mosaico "+" continua abrindo o seletor com X.
	if state.menuPanel == tabGames && state.gamesFocused {
		if circle && !prevCircle {
			state.gamesFocused = false
			state.confirmAction = ""
			redraw()
			return
		}
		if dpad != prevDpad {
			total := panelRows(tabGames)
			selected := state.row[tabGames]
			next := selected
			switch dpad {
			case 6, 5, 7: // esquerda
				next--
			case 2, 1, 3: // direita
				next++
			case 0: // cima
				next -= 5
			case 4: // baixo
				next += 5
			}
			if next >= 0 && next < total {
				state.row[tabGames] = next
			}
			state.confirmAction = ""
			redraw()
		}
		if triangle && !prevTriangle {
			openGameActionMenuLocked(state.row[tabGames])
			return
		}
		if cross && !prevCross {
			activateGameItemLocked(state.row[tabGames])
		}
		return
	}

	if triangle && !prevTriangle {
		beginOrganizingLocked()
		redraw()
		return
	}
	if circle && !prevCircle {
		closeMenuAfterCircleLocked(state.menuDevice)
		return
	}
	if dpad != prevDpad {
		if dpad == 6 || dpad == 5 || dpad == 7 {
			selectTabDeltaLocked(-1)
		}
		if dpad == 2 || dpad == 1 || dpad == 3 {
			selectTabDeltaLocked(1)
		}
		if state.menuPanel != tabGames {
			if dpad == 0 {
				rows := panelRows(state.menuPanel)
				if rows > 0 {
					state.row[state.menuPanel]--
					if state.row[state.menuPanel] < 0 {
						state.row[state.menuPanel] = rows - 1
					}
				}
			}
			if dpad == 4 {
				rows := panelRows(state.menuPanel)
				if rows > 0 {
					state.row[state.menuPanel]++
					if state.row[state.menuPanel] >= rows {
						state.row[state.menuPanel] = 0
					}
				}
			}
		}
		state.confirmAction = ""
		redraw()
	}
	if cross && !prevCross {
		if state.menuPanel == tabGames {
			state.gamesFocused = true
			total := panelRows(tabGames)
			if total > 0 {
				state.row[tabGames] = clamp(state.row[tabGames], 0, total-1)
			}
			redraw()
			return
		}
		activateSelectedItemLocked()
	}
}

func defaultTabOrder() []int {
	return []int{tabVolume, tabEnergy, tabModeGame, tabSettings, tabBattery, tabGames}
}

func sanitizeTabOrder(order []int) []int {
	if len(order) != tabCount {
		return defaultTabOrder()
	}
	seen := make([]bool, tabCount)
	out := make([]int, 0, tabCount)
	for _, id := range order {
		if id < 0 || id >= tabCount || seen[id] {
			return defaultTabOrder()
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func sanitizeGames(games []gameEntry) []gameEntry {
	out := make([]gameEntry, 0, len(games))
	seen := map[string]bool{}
	for _, game := range games {
		path := filepath.Clean(strings.TrimSpace(game.Path))
		if path == "." || path == "" || !strings.EqualFold(filepath.Ext(path), ".exe") {
			continue
		}
		key := strings.ToLower(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		name := strings.TrimSpace(game.Name)
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		out = append(out, gameEntry{Name: name, Path: path})
	}
	return out
}

func tabPositionLocked(id int) int {
	for i, tab := range state.tabOrder {
		if tab == id {
			return i
		}
	}
	return 0
}

func ensureSelectedTabVisibleLocked() {
	maxStart := tabCount - 5
	if maxStart < 0 {
		maxStart = 0
	}
	if state.menuPos < state.visibleStart {
		state.visibleStart = state.menuPos
	}
	if state.menuPos >= state.visibleStart+5 {
		state.visibleStart = state.menuPos - 4
	}
	state.visibleStart = clamp(state.visibleStart, 0, maxStart)
}

func selectTabDeltaLocked(delta int) {
	state.gameMenuOpen = false
	state.gamesFocused = false
	state.confirmAction = ""
	if len(state.tabOrder) != tabCount {
		state.tabOrder = defaultTabOrder()
	}
	newPos := state.menuPos + delta
	// Não faz loop entre a primeira e a última aba. Na primeira aba,
	// esquerda não executa nenhuma ação; na última, direita também não.
	if newPos < 0 || newPos >= tabCount {
		return
	}
	state.menuPos = newPos
	state.menuPanel = state.tabOrder[state.menuPos]
	rows := panelRows(state.menuPanel)
	if rows > 0 && state.row[state.menuPanel] >= rows {
		state.row[state.menuPanel] = rows - 1
	}
	ensureSelectedTabVisibleLocked()
}

func beginOrganizingLocked() {
	state.gameMenuOpen = false
	state.gamesFocused = false
	state.confirmAction = ""
	state.organizing = true
	state.organizeOriginal = append([]int(nil), state.tabOrder...)
	state.confirmAction = ""
}

func moveOrganizedTabLocked(delta int) {
	newPos := state.menuPos + delta
	if newPos < 0 || newPos >= tabCount {
		return
	}
	state.tabOrder[state.menuPos], state.tabOrder[newPos] = state.tabOrder[newPos], state.tabOrder[state.menuPos]
	state.menuPos = newPos
	state.menuPanel = state.tabOrder[state.menuPos]
	ensureSelectedTabVisibleLocked()
}

func finishOrganizingLocked(save bool) {
	if !state.organizing {
		return
	}
	currentTab := state.menuPanel
	if !save && len(state.organizeOriginal) == tabCount {
		state.tabOrder = append([]int(nil), state.organizeOriginal...)
		state.menuPos = tabPositionLocked(currentTab)
	}
	state.organizing = false
	state.organizeOriginal = nil
	ensureSelectedTabVisibleLocked()
	if save {
		saveSettingsLocked()
	}
}

func panelRows(p int) int {
	switch p {
	case tabVolume:
		return 3
	case tabEnergy, tabModeGame, tabSettings:
		return 4
	case tabBattery:
		return 1
	case tabGames:
		return len(state.games) + 1
	}
	return 0
}

func activateSelectedItemLocked() {
	p := state.menuPanel
	r := state.row[p]
	switch p {
	case tabVolume:
		if r == 0 {
			state.volumeAdjust = true
		} else if r == 1 {
			toggleMute()
		} else {
			state.audioOutputs = enumerateAudioOutputs()
			state.audioOutputIndex = currentAudioOutputIndex(state.audioOutputs)
			state.audioPicker = true
			redraw()
		}
	case tabEnergy:
		if r == 0 {
			requestPowerActionLocked("shutdown")
		} else if r == 1 {
			requestPowerActionLocked("restart")
		} else if r == 2 {
			requestPowerActionLocked("suspend")
		} else {
			cancelPendingShutdown()
		}
	case tabModeGame:
		if r == 0 {
			state.hideBatteryOverlay = !state.hideBatteryOverlay
		} else if r == 1 {
			state.batteryAlerts = !state.batteryAlerts
		} else if r == 2 {
			state.blockShutdown = !state.blockShutdown
		} else {
			desired := !state.showTaskbarIcon
			state.showTaskbarIcon = desired
			if err := applyTaskbarIconStyleLocked(); err != nil {
				state.showTaskbarIcon = !desired
				logf("falha ao alterar o ícone na barra de tarefas: %v", err)
				showMessageLocked("ÍCONE NA BARRA DE TAREFAS", "Não foi possível aplicar a alteração.", 2800*time.Millisecond)
			}
		}
		saveSettingsLocked()
		redraw()
	case tabSettings:
		if r == 0 {
			desired := !state.startupEnabled
			if err := setStartupEnabled(desired); err != nil {
				state.startupEnabled = isStartupEnabled()
				logf("falha ao alterar inicialização: %v", err)
				showMessageLocked("INICIAR COM O WINDOWS", "Não foi possível alterar a inicialização automática.", 2800*time.Millisecond)
			} else {
				state.startupEnabled = isStartupEnabled()
				if state.startupEnabled != desired {
					showMessageLocked("INICIAR COM O WINDOWS", "O Windows não confirmou a alteração.", 2800*time.Millisecond)
				}
			}
			saveSettingsLocked()
		} else if r == 1 {
			desired := !state.gameBarEnabled
			if err := setGameBarEnabledLocked(desired); err != nil {
				logf("falha ao alterar a Xbox Game Bar: %v", err)
				showMessageLocked("GAME BAR", "Não foi possível aplicar a alteração.", 2800*time.Millisecond)
			}
			saveSettingsLocked()
		} else if r == 2 {
			if state.warningSeconds == 0 {
				state.warningSeconds = 5
			} else if state.warningSeconds == 5 {
				state.warningSeconds = 10
			} else {
				state.warningSeconds = 0
			}
			saveSettingsLocked()
		} else {
			// Sobre o DualCenter agora permanece apenas informativo na própria linha da aba.
			// Nenhum overlay é aberto aqui para manter somente a versão do aplicativo visível.
		}
		redraw()
	case tabBattery:
		// A aba de bateria é informativa. O X não altera nenhuma configuração.
	case tabGames:
		activateGameItemLocked(r)
	}
}
