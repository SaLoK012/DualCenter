//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeInstallDirAppendsProductFolder(t *testing.T) {
	base := t.TempDir()
	got, err := normalizeInstallDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Base(got), productName) {
		t.Fatalf("destino = %q; deveria terminar em %q", got, productName)
	}
}

func TestNormalizeInstallDirRejectsUnknownContents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), productName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "arquivo-do-usuario.txt"), []byte("preservar"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeInstallDir(dir); err == nil {
		t.Fatal("pasta com conteúdo desconhecido deveria ser recusada")
	}
}

func TestInstallMarkerLimitsOwnedFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), productName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeInstallMarker(dir); err != nil {
		t.Fatal(err)
	}
	marker, err := validateUninstallDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if marker.ProductID != installProductID || len(marker.Files) != 3 {
		t.Fatalf("marcador inesperado: %#v", marker)
	}
	if err := os.WriteFile(filepath.Join(dir, "preservar.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	unexpected, err := directoryContainsUnexpectedFiles(dir, marker)
	if err != nil {
		t.Fatal(err)
	}
	if len(unexpected) != 1 || unexpected[0] != "preservar.txt" {
		t.Fatalf("arquivos inesperados = %v", unexpected)
	}
}

func TestReadInstallMarkerRejectsInjectedFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), productName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := newInstallMarker()
	marker.Files = append(marker.Files, "arquivo-do-usuario.txt")
	data, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath(dir), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readInstallMarker(dir); err == nil {
		t.Fatal("marcador adulterado deveria ser recusado")
	}
}

func TestWriteFileAtomicallyReplacesStaleTemporaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DualCenter.exe")
	if err := os.WriteFile(path+".new", []byte("resíduo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeFileAtomically(path, strings.NewReader("conteúdo novo"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "conteúdo novo" {
		t.Fatalf("conteúdo = %q; esperado conteúdo novo", data)
	}
	if _, err := os.Stat(path + ".new"); !os.IsNotExist(err) {
		t.Fatalf("arquivo temporário não foi limpo: %v", err)
	}
}
