//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

const (
	gameBarPath = `Software\Microsoft\GameBar`
	gameBarName = "UseNexusForGameBarEnabled"
)

type gameBarSettingsBackup struct {
	Captured bool   `json:"gameBarBackupCaptured"`
	Exists   bool   `json:"gameBarOriginalExists"`
	Value    uint32 `json:"gameBarOriginalValue"`
}

func saveInstallerGameBarChoice(enabled bool) error {
	dir := localAppDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.json")
	document := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil {
		if unmarshalErr := json.Unmarshal(data, &document); unmarshalErr != nil {
			corrupt := path + "." + time.Now().Format("20060102-150405") + ".corrupt"
			if renameErr := os.Rename(path, corrupt); renameErr != nil {
				return fmt.Errorf("configuração existente inválida e não pôde ser preservada: %w", renameErr)
			}
			document = map[string]any{}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	version, hasVersion := document["version"].(float64)
	if !hasVersion || version < 4 {
		document["showTaskbarIcon"] = true
	}
	document["version"] = 4
	document["gameBarEnabled"] = enabled
	document["gameBarChoiceMade"] = true

	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	_, err = writeFileAtomically(path, strings.NewReader(string(updated)), 0o644)
	return err
}

func deleteRegValue(root uintptr, path, name string) error {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	r, _, _ = pRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(name))))
	if r != 0 && r != 2 {
		return fmt.Errorf("RegDeleteValueW=%d", r)
	}
	return nil
}

func restoreGameBarFromSettings() error {
	path := filepath.Join(localAppDataDir(), "settings.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var backup gameBarSettingsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("configuração da Xbox Game Bar inválida: %w", err)
	}
	if !backup.Captured {
		return nil
	}
	if backup.Exists {
		if err := setRegDWORD(0x80000001, gameBarPath, gameBarName, backup.Value); err != nil {
			return err
		}
	} else if err := deleteRegValue(0x80000001, gameBarPath, gameBarName); err != nil {
		return err
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err == nil {
		document["gameBarBackupCaptured"] = false
		document["gameBarOriginalExists"] = false
		document["gameBarOriginalValue"] = 0
		document["gameBarChoiceMade"] = false
		document["gameBarEnabled"] = backup.Exists && backup.Value != 0
		updated, marshalErr := json.MarshalIndent(document, "", "  ")
		if marshalErr == nil {
			updated = append(updated, '\n')
			_, _ = writeFileAtomically(path, strings.NewReader(string(updated)), 0o644)
		}
	}
	return nil
}
