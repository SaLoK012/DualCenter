//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBatteryStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   byte
		known    bool
		percent  int
		charging bool
	}{
		{name: "empty", status: 0x00, known: true, percent: 5},
		{name: "discharging", status: 0x09, known: true, percent: 95},
		{name: "charging", status: 0x14, known: true, percent: 45, charging: true},
		{name: "full", status: 0x2f, known: true, percent: 100, charging: true},
		{name: "unknown", status: 0x30},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			known, percent, charging := parseBatteryStatus(test.status)
			if known != test.known || percent != test.percent || charging != test.charging {
				t.Fatalf("parseBatteryStatus(%#x) = (%t, %d, %t), want (%t, %d, %t)", test.status, known, percent, charging, test.known, test.percent, test.charging)
			}
		})
	}
}

func TestDualSenseIdentificationFromPath(t *testing.T) {
	standard := `\\?\HID#VID_054C&PID_0CE6`
	edge := `\\?\HID#VID_054C&PID_0DF2`
	if !rawPathLooksDualSense(standard) || dualSenseModelFromPath(standard) != controllerModelStandard {
		t.Fatal("standard DualSense path was not identified")
	}
	if !rawPathLooksDualSense(edge) || dualSenseModelFromPath(edge) != controllerModelEdge {
		t.Fatal("DualSense Edge path was not identified")
	}
	if rawPathLooksDualSense(`\\?\HID#VID_1234&PID_0CE6`) {
		t.Fatal("non-Sony controller was incorrectly identified")
	}
}

func TestGameDialogSuspendsMenuControllerInput(t *testing.T) {
	originalMode := state.overlayMode
	originalDevice := state.menuDevice
	originalDialogOpen := state.gameDialogOpen
	defer func() {
		state.overlayMode = originalMode
		state.menuDevice = originalDevice
		state.gameDialogOpen = originalDialogOpen
	}()

	const device uintptr = 42
	state.overlayMode = overlayMenu
	state.menuDevice = device
	state.gameDialogOpen = false
	if !menuControllerInputEnabledLocked(device) {
		t.Fatal("menu input should be enabled for the controller that opened it")
	}

	state.gameDialogOpen = true
	if menuControllerInputEnabledLocked(device) {
		t.Fatal("menu input must be suspended while the game file dialog is open")
	}
	if menuControllerInputEnabledLocked(device + 1) {
		t.Fatal("another controller must not control the menu")
	}
}

func TestAudioPickerNormalizesStaleSelection(t *testing.T) {
	originalOutputs := state.audioOutputs
	originalIndex := state.audioOutputIndex
	defer func() {
		state.audioOutputs = originalOutputs
		state.audioOutputIndex = originalIndex
	}()

	state.audioOutputs = []audioDevice{{ID: "device", Name: "Saída"}}
	for _, staleIndex := range []int{-1, 4} {
		state.audioOutputIndex = staleIndex
		handleAudioPickerLocked(8, 8, false, false, false, false)
		if state.audioOutputIndex != 0 {
			t.Fatalf("índice obsoleto %d foi normalizado para %d; esperado 0", staleIndex, state.audioOutputIndex)
		}
	}
}

func TestGameStateSnapshotRejectsStaleLibrary(t *testing.T) {
	originalGames := state.games
	originalRunning := state.gameRunning
	defer func() {
		state.games = originalGames
		state.gameRunning = originalRunning
	}()

	const current = `C:\Jogos\Atual.exe`
	state.games = []gameEntry{{Name: "Atual", Path: current}}
	state.gameRunning = []bool{false}
	key := strings.ToLower(filepath.Clean(current))
	applied, changed := applyGameStateSnapshotLocked([]string{key}, map[string]bool{key: true})
	if !applied || !changed || !state.gameRunning[0] {
		t.Fatalf("snapshot válido não foi aplicado: applied=%t changed=%t running=%v", applied, changed, state.gameRunning)
	}

	applied, changed = applyGameStateSnapshotLocked([]string{strings.ToLower(filepath.Clean(`C:\Jogos\Antigo.exe`))}, nil)
	if applied || changed || !state.gameRunning[0] {
		t.Fatalf("snapshot obsoleto alterou o estado: applied=%t changed=%t running=%v", applied, changed, state.gameRunning)
	}
}
