//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func initPaths() {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			base = filepath.Join(profile, "AppData", "Local")
		} else {
			base = os.TempDir()
		}
	}
	appDataDir = filepath.Join(base, "DualCenter")
	_ = os.MkdirAll(appDataDir, 0o755)
	settingsPath = filepath.Join(appDataDir, "settings.json")
	logPath = filepath.Join(appDataDir, "DualCenter.log")
}

func initLog() {
	if logPath == "" {
		initPaths()
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
}

func loadSettings() {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return
	}
	var cfg settingsFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		logf("settings inválido: %v", err)
		corrupt := fmt.Sprintf("%s.%s.corrupt", settingsPath, time.Now().Format("20060102-150405"))
		if renameErr := os.Rename(settingsPath, corrupt); renameErr != nil {
			logf("não foi possível preservar settings inválido: %v", renameErr)
		} else {
			settingsRecovered = true
		}
		return
	}
	legacySettingsLoaded = cfg.Version > 0 && cfg.Version < 4
	state.hideBatteryOverlay = cfg.HideBatteryOverlay
	state.batteryAlerts = cfg.BatteryAlerts
	state.blockShutdown = cfg.BlockShutdown
	if cfg.Version >= 4 {
		state.showTaskbarIcon = cfg.ShowTaskbarIcon
		state.gameBarEnabled = cfg.GameBarEnabled
		state.gameBarChoiceMade = cfg.GameBarChoiceMade
		state.gameBarBackup = gameBarBackup{
			captured: cfg.GameBarBackupCaptured,
			exists:   cfg.GameBarOriginalExists,
			value:    cfg.GameBarOriginalValue,
		}
	} else {
		state.showTaskbarIcon = true
		state.gameBarEnabled = false
		state.gameBarChoiceMade = false
	}
	state.tabOrder = sanitizeTabOrder(cfg.TabOrder)
	state.games = sanitizeGames(cfg.Games)
	state.gameRunning = make([]bool, len(state.games))
	state.menuPanel = state.tabOrder[0]
	state.menuPos = 0
	switch cfg.WarningSeconds {
	case 0, 5, 10:
		state.warningSeconds = cfg.WarningSeconds
	}
}

func saveSettingsLocked() {
	if settingsPath == "" {
		return
	}
	cfg := settingsFile{
		Version:               4,
		HideBatteryOverlay:    state.hideBatteryOverlay,
		BatteryAlerts:         state.batteryAlerts,
		BlockShutdown:         state.blockShutdown,
		ShowTaskbarIcon:       state.showTaskbarIcon,
		GameBarEnabled:        state.gameBarEnabled,
		GameBarChoiceMade:     state.gameBarChoiceMade,
		GameBarBackupCaptured: state.gameBarBackup.captured,
		GameBarOriginalExists: state.gameBarBackup.exists,
		GameBarOriginalValue:  state.gameBarBackup.value,
		WarningSeconds:        state.warningSeconds,
		TabOrder:              append([]int(nil), state.tabOrder...),
		Games:                 append([]gameEntry(nil), state.games...),
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logf("falha ao serializar settings: %v", err)
		return
	}
	b = append(b, '\n')
	if err := replaceFileAtomically(settingsPath, b, 0o644); err != nil {
		logf("falha ao substituir settings: %v", err)
	}
}

func logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	rotateLogIfNeeded(logPath, maxLogSize)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s  %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
}
