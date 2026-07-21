//go:build windows

package main

import (
	"os"
	"strings"
	"time"
	"unsafe"
)

func hasCommandLineFlag(name string) bool {
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(strings.TrimSpace(arg), name) {
			return true
		}
	}
	return false
}

// handleMaintenanceCommand executa comandos curtos usados pelo instalador e
// pelo suporte sem iniciar a interface residente.
func handleMaintenanceCommand() bool {
	if hasCommandLineFlag("--exit") {
		hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(utf16Ptr(windowClassName))), 0)
		if hwnd != 0 {
			pPostMessageW.Call(hwnd, WM_CLOSE, 0, 0)
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				hwnd, _, _ = pFindWindowW.Call(uintptr(unsafe.Pointer(utf16Ptr(windowClassName))), 0)
				if hwnd == 0 {
					break
				}
			}
		}
		return true
	}

	if hasCommandLineFlag("--restore-gamebar") {
		state.mu.Lock()
		err := restoreOriginalGameBarLocked()
		if err == nil {
			saveSettingsLocked()
		}
		state.mu.Unlock()
		if err != nil {
			logf("falha ao restaurar a Xbox Game Bar: %v", err)
		}
		return true
	}
	return false
}
