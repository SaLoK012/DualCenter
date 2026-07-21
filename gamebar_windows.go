//go:build windows

package main

import (
	"fmt"
	"unsafe"
)

const (
	gameBarRegistryPath = `Software\Microsoft\GameBar`
	gameBarRegistryName = "UseNexusForGameBarEnabled"
)

type gameBarBackup struct {
	captured bool
	exists   bool
	value    uint32
}

func readGameBarValue() (uint32, bool, error) {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(
		0x80000001,
		uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryPath))),
		0, 0, 0,
		KEY_QUERY_VALUE|KEY_SET_VALUE,
		0,
		uintptr(unsafe.Pointer(&key)),
		0,
	)
	if r != 0 {
		return 0, false, fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)

	var value, typ, size uint32
	size = uint32(unsafe.Sizeof(value))
	r, _, _ = pRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryName))),
		0,
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == ERROR_FILE_NOT_FOUND {
		return 0, false, nil
	}
	if r != 0 {
		return 0, false, fmt.Errorf("RegQueryValueExW=%d", r)
	}
	if typ != REG_DWORD || size != 4 {
		return 0, false, fmt.Errorf("valor da Xbox Game Bar possui formato inesperado")
	}
	return value, true, nil
}

func writeGameBarValue(value uint32) error {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(
		0x80000001,
		uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryPath))),
		0, 0, 0,
		KEY_QUERY_VALUE|KEY_SET_VALUE,
		0,
		uintptr(unsafe.Pointer(&key)),
		0,
	)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	r, _, _ = pRegSetValueExW.Call(
		key,
		uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryName))),
		0,
		REG_DWORD,
		uintptr(unsafe.Pointer(&value)),
		unsafe.Sizeof(value),
	)
	if r != 0 {
		return fmt.Errorf("RegSetValueExW=%d", r)
	}
	return nil
}

func deleteGameBarValue() error {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(
		0x80000001,
		uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryPath))),
		0, 0, 0,
		KEY_SET_VALUE,
		0,
		uintptr(unsafe.Pointer(&key)),
		0,
	)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	r, _, _ = pRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(gameBarRegistryName))))
	if r != 0 && r != ERROR_FILE_NOT_FOUND {
		return fmt.Errorf("RegDeleteValueW=%d", r)
	}
	return nil
}

func captureGameBarBackupLocked() error {
	if state.gameBarBackup.captured {
		return nil
	}
	if legacySettingsLoaded {
		// Versões anteriores sempre gravavam zero e não preservavam o estado.
		// Restaurar um após a remoção é a alternativa que devolve o botão ao Windows.
		state.gameBarBackup = gameBarBackup{captured: true, exists: true, value: 1}
		return nil
	}
	value, exists, err := readGameBarValue()
	if err != nil {
		return err
	}
	state.gameBarBackup = gameBarBackup{captured: true, exists: exists, value: value}
	return nil
}

func setGameBarEnabledLocked(enabled bool) error {
	if err := captureGameBarBackupLocked(); err != nil {
		return err
	}
	value := uint32(0)
	if enabled {
		value = 1
	}
	if err := writeGameBarValue(value); err != nil {
		return err
	}
	state.gameBarEnabled = enabled
	return nil
}

func restoreOriginalGameBarLocked() error {
	if !state.gameBarBackup.captured {
		return nil
	}
	var err error
	if state.gameBarBackup.exists {
		err = writeGameBarValue(state.gameBarBackup.value)
	} else {
		err = deleteGameBarValue()
	}
	if err != nil {
		return err
	}
	state.gameBarBackup = gameBarBackup{}
	return nil
}

func initializeGameBarChoice() {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.gameBarChoiceMade {
		body := "A Game Bar usa o mesmo botão central do controle que o DualCenter.\n\n" +
			"Deseja manter a Game Bar ligada?\n\n" +
			"Sim: a Game Bar continua ligada.\n" +
			"Não: o DualCenter impede que ela abra junto com o botão PS."
		answer, _, _ := pMessageBoxW.Call(
			0,
			uintptr(unsafe.Pointer(utf16Ptr(body))),
			uintptr(unsafe.Pointer(utf16Ptr("DualCenter — Game Bar"))),
			0x00000004|0x00000020,
		)
		state.gameBarEnabled = answer == 6
		state.gameBarChoiceMade = true
	}

	if err := setGameBarEnabledLocked(state.gameBarEnabled); err != nil {
		logf("falha ao configurar a Xbox Game Bar: %v", err)
		showFatal("Game Bar", "Não foi possível aplicar a preferência da Game Bar. O DualCenter continuará funcionando.")
	}
	saveSettingsLocked()
}
