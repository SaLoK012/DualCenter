//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	installMarkerName = ".dualcenter-install.json"
	installProductID  = "com.salok.dualcenter"
)

type installMarker struct {
	ProductID   string   `json:"productId"`
	ProductName string   `json:"productName"`
	Version     string   `json:"version"`
	InstalledAt string   `json:"installedAt"`
	Files       []string `json:"files"`
}

func markerPath(dir string) string {
	return filepath.Join(dir, installMarkerName)
}

func normalizeInstallDir(input string) (string, error) {
	input = strings.Trim(strings.TrimSpace(input), `"`)
	if input == "" {
		return "", fmt.Errorf("selecione uma pasta de instalação")
	}
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("caminho inválido: %w", err)
	}
	abs = filepath.Clean(abs)
	volume := filepath.VolumeName(abs)
	if volume == "" {
		return "", fmt.Errorf("escolha uma unidade local válida")
	}
	root := filepath.Clean(volume + `\`)
	if strings.EqualFold(abs, root) {
		return "", fmt.Errorf("não é permitido instalar diretamente na raiz da unidade")
	}

	if !strings.EqualFold(filepath.Base(abs), productName) {
		abs = filepath.Join(abs, productName)
	}

	if windowsDir := strings.TrimSpace(os.Getenv("WINDIR")); windowsDir != "" {
		windowsDir = filepath.Clean(windowsDir)
		if strings.EqualFold(abs, windowsDir) || pathWithin(abs, windowsDir) {
			return "", fmt.Errorf("não é permitido instalar dentro da pasta do Windows")
		}
	}

	info, err := os.Lstat(abs)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("o caminho selecionado não é uma pasta")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("atalhos e links de pasta não são permitidos como destino")
		}
		if err := validateExistingInstallDir(abs); err != nil {
			return "", err
		}
	}
	return abs, nil
}

func pathWithin(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, `..\`) && !strings.HasPrefix(rel, "../")
}

func validateExistingInstallDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if marker, err := readInstallMarker(dir); err == nil && marker.ProductID == installProductID {
		return nil
	}

	legacyAllowed := map[string]bool{
		strings.ToLower("DualCenter.exe"): true,
		strings.ToLower("Uninstall.exe"):  true,
	}
	for _, entry := range entries {
		if entry.IsDir() || !legacyAllowed[strings.ToLower(entry.Name())] {
			return fmt.Errorf("a pasta já contém arquivos que não pertencem a uma instalação reconhecida do DualCenter")
		}
	}
	return nil
}

func newInstallMarker() installMarker {
	return installMarker{
		ProductID:   installProductID,
		ProductName: productName,
		Version:     productVersion(),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Files:       []string{"DualCenter.exe", "Uninstall.exe", installMarkerName},
	}
}

func writeInstallMarker(dir string) error {
	data, err := json.MarshalIndent(newInstallMarker(), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = writeFileAtomically(markerPath(dir), strings.NewReader(string(data)), 0o644)
	return err
}

func readInstallMarker(dir string) (installMarker, error) {
	data, err := os.ReadFile(markerPath(dir))
	if err != nil {
		return installMarker{}, err
	}
	var marker installMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return installMarker{}, err
	}
	if marker.ProductID != installProductID || marker.ProductName != productName {
		return installMarker{}, fmt.Errorf("marcador de instalação não pertence ao DualCenter")
	}
	expected := map[string]bool{
		strings.ToLower("DualCenter.exe"):  true,
		strings.ToLower("Uninstall.exe"):   true,
		strings.ToLower(installMarkerName): true,
	}
	if len(marker.Files) != len(expected) {
		return installMarker{}, fmt.Errorf("lista de arquivos do marcador é inválida")
	}
	seen := make(map[string]bool, len(marker.Files))
	for _, name := range marker.Files {
		clean := filepath.Clean(name)
		base := strings.ToLower(filepath.Base(clean))
		if clean != filepath.Base(clean) || !expected[base] || seen[base] {
			return installMarker{}, fmt.Errorf("arquivo não autorizado no marcador: %q", name)
		}
		seen[base] = true
	}
	return marker, nil
}

func validateUninstallDir(dir string) (installMarker, error) {
	dir = filepath.Clean(dir)
	volume := filepath.VolumeName(dir)
	if volume == "" || strings.EqualFold(dir, filepath.Clean(volume+`\`)) {
		return installMarker{}, fmt.Errorf("pasta de desinstalação insegura")
	}
	if !strings.EqualFold(filepath.Base(dir), productName) {
		return installMarker{}, fmt.Errorf("a pasta de instalação não possui o nome esperado")
	}
	marker, err := readInstallMarker(dir)
	if err != nil {
		return installMarker{}, fmt.Errorf("instalação não reconhecida: %w", err)
	}
	return marker, nil
}

func removeOwnedFilesAfterExit(dir string, marker installMarker, pid int) error {
	files := make([]string, 0, len(marker.Files))
	for _, name := range marker.Files {
		name = filepath.Base(filepath.Clean(name))
		if name == "." || name == string(filepath.Separator) || name == "" {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	quoted := make([]string, 0, len(files))
	for _, path := range files {
		quoted = append(quoted, "'"+strings.ReplaceAll(path, "'", "''")+"'")
	}
	quotedDir := strings.ReplaceAll(dir, "'", "''")
	script := fmt.Sprintf(
		`$ErrorActionPreference='SilentlyContinue'; while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 100 }; Remove-Item -LiteralPath @(%s) -Force -ErrorAction SilentlyContinue; Remove-Item -LiteralPath '%s' -Force -ErrorAction SilentlyContinue`,
		pid,
		strings.Join(quoted, ","),
		quotedDir,
	)
	cmd := hiddenCommandDetached("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	return cmd
}

func directoryContainsUnexpectedFiles(dir string, marker installMarker) ([]string, error) {
	owned := map[string]bool{}
	for _, name := range marker.Files {
		owned[strings.ToLower(filepath.Base(name))] = true
	}
	var unexpected []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if entry.IsDir() || !owned[strings.ToLower(filepath.Base(rel))] || strings.Contains(rel, string(filepath.Separator)) {
			unexpected = append(unexpected, rel)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return unexpected, err
}
