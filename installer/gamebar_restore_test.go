//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readInstallerSettings(t *testing.T, base string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, productName, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestSaveInstallerGameBarChoiceCreatesSettings(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	if err := saveInstallerGameBarChoice(false); err != nil {
		t.Fatal(err)
	}
	document := readInstallerSettings(t, base)
	if document["version"] != float64(4) || document["gameBarChoiceMade"] != true || document["gameBarEnabled"] != false {
		t.Fatalf("preferência inesperada: %#v", document)
	}
	if document["showTaskbarIcon"] != true {
		t.Fatal("uma configuração nova deve manter o ícone da barra de tarefas habilitado")
	}
}

func TestSaveInstallerGameBarChoicePreservesBackup(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	dir := filepath.Join(base, productName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte(`{"version":4,"gameBarBackupCaptured":true,"gameBarOriginalExists":true,"gameBarOriginalValue":1,"warningSeconds":10}`)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveInstallerGameBarChoice(true); err != nil {
		t.Fatal(err)
	}
	document := readInstallerSettings(t, base)
	if document["gameBarBackupCaptured"] != true || document["gameBarOriginalValue"] != float64(1) {
		t.Fatalf("backup da Game Bar não foi preservado: %#v", document)
	}
	if document["warningSeconds"] != float64(10) || document["gameBarEnabled"] != true {
		t.Fatalf("configurações existentes não foram preservadas: %#v", document)
	}
}
