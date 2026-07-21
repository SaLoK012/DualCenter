// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"dualcenter/internal/version"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

func stopRunningDualCenter() {
	className := utf16Ptr("DualCenterOverlayWindow")
	hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	if hwnd == 0 {
		return
	}
	var pid uint32
	pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	pPostMessageW.Call(hwnd, wmClose, 0, 0)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		hwnd, _, _ = pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if hwnd == 0 {
			return
		}
	}
	// Fallback restrito ao PID da janela validada; nunca encerra outro processo
	// apenas porque ele possui o mesmo nome de arquivo.
	if pid != 0 {
		const processTerminate = 0x0001
		process, _, _ := pOpenProcess.Call(processTerminate, 0, uintptr(pid))
		if process != 0 {
			pTerminateProcess.Call(process, 1)
			pWaitForSingleObject.Call(process, 3000)
			pCloseHandle.Call(process)
		}
	}
}

func writeFileAtomically(path string, source io.Reader, perm os.FileMode) (int64, error) {
	tmp := path + ".new"
	backup := path + ".previous"
	_ = os.Remove(tmp)
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(file, source)
	var syncErr error
	if copyErr == nil {
		syncErr = file.Sync()
	}
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return written, copyErr
	}
	if syncErr != nil {
		_ = os.Remove(tmp)
		return written, syncErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return written, closeErr
	}
	hadPrevious := false
	if _, statErr := os.Stat(path); statErr == nil {
		_ = os.Remove(backup)
		if err := os.Rename(path, backup); err != nil {
			_ = os.Remove(tmp)
			return written, err
		}
		hadPrevious = true
	} else if !os.IsNotExist(statErr) {
		_ = os.Remove(tmp)
		return written, statErr
	}
	if err := os.Rename(tmp, path); err != nil {
		if hadPrevious {
			_ = os.Rename(backup, path)
		}
		_ = os.Remove(tmp)
		return written, err
	}
	if hadPrevious {
		_ = os.Remove(backup)
	}
	return written, nil
}

func createFont(height int32, weight int32, face string) uintptr {
	h, _, _ := pCreateFontW.Call(
		uintptr(scaleFont(height)), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16Ptr(face))),
	)
	return h
}

func createControl(className, text string, style, exStyle uintptr, x, y, w, height int32, id uintptr) uintptr {
	// A janela principal e os textos são desenhados em pixels reais quando o
	// instalador está DPI-aware. Os controles nativos precisam usar a mesma
	// escala; se ficarem em coordenadas 96-DPI, em 4K/200% eles sobem para o
	// cabeçalho e parecem "amassados" no canto esquerdo.
	hwnd, _, _ := pCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(utf16Ptr(className))),
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		style,
		uintptr(scale(x)), uintptr(scale(y)), uintptr(scale(w)), uintptr(scale(height)),
		hwndMain,
		id,
		0,
		0,
	)
	if fontNormal != 0 {
		pSendMessageW.Call(hwnd, wmSetFont, fontNormal, 1)
	}
	return hwnd
}

func setWindowText(hwnd uintptr, text string) {
	pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func getWindowText(hwnd uintptr) string {
	l, _, _ := pGetWindowTextLengthW.Call(hwnd)
	buf := make([]uint16, int(l)+1)
	if len(buf) == 0 {
		return ""
	}
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return utf16String(buf)
}

func checked(hwnd uintptr) bool {
	r, _, _ := pSendMessageW.Call(hwnd, bmGetCheck, 0, 0)
	return r == bstChecked
}

func setChecked(hwnd uintptr, value bool) {
	v := uintptr(0)
	if value {
		v = bstChecked
	}
	pSendMessageW.Call(hwnd, bmSetCheck, v, 0)
}

func setInstallError(message string) {
	installErrMu.Lock()
	lastInstallErr = message
	installErrMu.Unlock()
}

func installError() string {
	installErrMu.Lock()
	defer installErrMu.Unlock()
	return lastInstallErr
}

func setOperationWarning(message string) {
	installErrMu.Lock()
	lastWarning = message
	installErrMu.Unlock()
}

func operationWarning() string {
	installErrMu.Lock()
	defer installErrMu.Unlock()
	return lastWarning
}

func updateProgress(percent int, text string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	progressMu.Lock()
	progressPercent = percent
	progressStatus = text
	progressMu.Unlock()
	if hwndMain != 0 {
		pPostMessageW.Call(hwndMain, wmAppProgress, 0, 0)
	}
}

func applyProgress() {
	progressMu.Lock()
	percent := progressPercent
	text := progressStatus
	progressMu.Unlock()
	if hProgress != 0 {
		pSendMessageW.Call(hProgress, pbmSetPos, uintptr(percent), 0)
	}
	if hStatus != 0 {
		setWindowText(hStatus, text)
	}
}

func showControl(hwnd uintptr, visible bool) {
	if hwnd == 0 {
		return
	}
	if visible {
		pShowWindow.Call(hwnd, swShow)
	} else {
		pShowWindow.Call(hwnd, swHide)
	}
}

func showSetupPage() {
	statePage = pageSetup
	isInstall := windowMode == modeInstall
	showControl(hEditDir, isInstall)
	showControl(hBrowse, isInstall)
	showControl(hDesktop, isInstall)
	showControl(hLaunch, isInstall)
	showControl(hGameBar, isInstall)
	showControl(hRemoveSettings, !isInstall)
	showControl(hProgress, false)
	showControl(hStatus, false)
	showControl(hCancel, true)
	showControl(hInstall, true)
	if isInstall {
		setWindowText(hInstall, "Instalar agora")
	} else {
		setWindowText(hInstall, "Desinstalar")
	}
	setWindowText(hCancel, "Cancelar")
	pInvalidateRect.Call(hwndMain, 0, 1)
}

func showProgressPage() {
	statePage = pageProgress
	showControl(hEditDir, false)
	showControl(hBrowse, false)
	showControl(hDesktop, false)
	showControl(hLaunch, false)
	showControl(hGameBar, false)
	showControl(hRemoveSettings, false)
	showControl(hProgress, true)
	showControl(hStatus, true)
	showControl(hCancel, false)
	showControl(hInstall, false)
	pInvalidateRect.Call(hwndMain, 0, 1)
}

func installDir() string {
	base := os.Getenv("ProgramFiles")
	if base == "" {
		base = `C:\Program Files`
	}
	return filepath.Join(base, productName)
}

func localAppDataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			base = filepath.Join(profile, "AppData", "Local")
		} else {
			base = os.TempDir()
		}
	}
	return filepath.Join(base, productName)
}

func startMenuShortcut() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, `Microsoft\Windows\Start Menu\Programs`, productName+".lnk")
}

func desktopShortcut() string {
	if public := os.Getenv("PUBLIC"); public != "" {
		return filepath.Join(public, "Desktop", productName+".lnk")
	}
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		return filepath.Join(profile, "Desktop", productName+".lnk")
	}
	return filepath.Join(os.TempDir(), productName+".lnk")
}

func setRegString(root uintptr, path, name, value string) error {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	r, _, _ = pRegSetValueExW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, regSZ, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if r != 0 {
		return fmt.Errorf("RegSetValueExW(%s)=%d", name, r)
	}
	return nil
}

func setRegDWORD(root uintptr, path, name string, value uint32) error {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	r, _, _ = pRegSetValueExW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(name))), 0, regDWORD, uintptr(unsafe.Pointer(&value)), 4)
	if r != 0 {
		return fmt.Errorf("RegSetValueExW(%s)=%d", name, r)
	}
	return nil
}

func removeRegTree(root uintptr, parent, child string) {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(utf16Ptr(parent))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return
	}
	defer pRegCloseKey.Call(key)
	pRegDeleteTreeW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(child))))
}

func removeStartupValue() {
	const path = `Software\Microsoft\Windows\CurrentVersion\Run`
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(0x80000001, uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return
	}
	defer pRegCloseKey.Call(key)
	pRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(productName))))
}

func registerUninstaller(dir string, sizeBytes int64) error {
	const key = `Software\Microsoft\Windows\CurrentVersion\Uninstall\DualCenter`
	uninstaller := filepath.Join(dir, "Uninstall.exe")
	app := filepath.Join(dir, "DualCenter.exe")
	installDate := time.Now().Format("20060102")
	values := map[string]string{
		"DisplayName":          productName,
		"DisplayVersion":       productVersion(),
		"Publisher":            version.Current.Publisher,
		"InstallLocation":      dir,
		"InstallDate":          installDate,
		"UninstallString":      `"` + uninstaller + `" --uninstall`,
		"QuietUninstallString": `"` + uninstaller + `" --uninstall --quiet`,
		"DisplayIcon":          app + ",0",
	}
	for name, value := range values {
		if err := setRegString(0x80000002, key, name, value); err != nil {
			return err
		}
	}
	if err := setRegDWORD(0x80000002, key, "EstimatedSize", uint32((sizeBytes+1023)/1024)); err != nil {
		return err
	}
	if err := setRegDWORD(0x80000002, key, "NoModify", 1); err != nil {
		return err
	}
	if err := setRegDWORD(0x80000002, key, "NoRepair", 1); err != nil {
		return err
	}
	return nil
}

type installOptions struct {
	dir             string
	createDesktop   bool
	launchAfterDone bool
	gameBarEnabled  bool
}

func installWithProgress(opts installOptions) error {
	dir, err := normalizeInstallDir(opts.dir)
	if err != nil {
		return err
	}
	updateProgress(3, "Preparando instalação...")

	updateProgress(8, "Solicitando o fechamento do DualCenter, se estiver aberto...")
	stopRunningDualCenter()

	updateProgress(14, "Criando pasta de instalação...")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	updateProgress(22, "Abrindo pacote do DualCenter...")
	payload, err := payloadFS.Open("payload/DualCenter.exe")
	if err != nil {
		return err
	}
	defer payload.Close()
	payloadInfo, err := payload.Stat()
	if err != nil {
		return err
	}

	appPath := filepath.Join(dir, "DualCenter.exe")
	updateProgress(30, "Instalando arquivo principal: DualCenter.exe")
	if _, err := writeFileAtomically(appPath, payload, 0o755); err != nil {
		return err
	}

	updateProgress(70, "Criando desinstalador...")
	self, err := os.Executable()
	if err != nil {
		return err
	}
	selfFile, err := os.Open(self)
	if err != nil {
		return err
	}
	defer selfFile.Close()
	selfInfo, err := selfFile.Stat()
	if err != nil {
		return err
	}
	uninstallPath := filepath.Join(dir, "Uninstall.exe")
	if _, err := writeFileAtomically(uninstallPath, selfFile, 0o755); err != nil {
		return err
	}
	if err := writeInstallMarker(dir); err != nil {
		return fmt.Errorf("não foi possível registrar os arquivos da instalação: %w", err)
	}

	updateProgress(78, "Registrando no Windows Apps e Recursos...")
	if err := registerUninstaller(dir, payloadInfo.Size()+selfInfo.Size()); err != nil {
		return err
	}

	updateProgress(84, "Criando atalho no Menu Iniciar...")
	if err := createShortcutAt(startMenuShortcut(), appPath); err != nil {
		return err
	}
	if opts.createDesktop {
		updateProgress(90, "Criando atalho na Área de Trabalho...")
		if err := createShortcutAt(desktopShortcut(), appPath); err != nil {
			return err
		}
	} else {
		_ = os.Remove(desktopShortcut())
	}

	updateProgress(94, "Salvando preferência da Xbox Game Bar...")
	if err := saveInstallerGameBarChoice(opts.gameBarEnabled); err != nil {
		return fmt.Errorf("não foi possível salvar a preferência da Game Bar: %w", err)
	}

	updateProgress(98, "Finalizando a instalação...")
	updateProgress(100, "Instalação concluída com sucesso.")
	return nil
}

func launchInstalledApp(dir string) error {
	appPath := filepath.Join(dir, "DualCenter.exe")
	// O Explorer já executa na sessão normal do usuário e evita que o aplicativo
	// herde a elevação administrativa usada pelo instalador.
	cmd := exec.Command("explorer.exe", appPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

type uninstallPlan struct {
	dir        string
	marker     installMarker
	unexpected []string
}

func prepareUninstall() (uninstallPlan, error) {
	self, err := os.Executable()
	if err != nil {
		return uninstallPlan{}, fmt.Errorf("não foi possível identificar o desinstalador: %w", err)
	}
	dir := filepath.Dir(self)
	marker, err := validateUninstallDir(dir)
	if err != nil {
		return uninstallPlan{}, fmt.Errorf("a instalação não pôde ser validada: %w", err)
	}
	unexpected, scanErr := directoryContainsUnexpectedFiles(dir, marker)
	if scanErr != nil {
		return uninstallPlan{}, fmt.Errorf("não foi possível conferir com segurança os arquivos da pasta: %w", scanErr)
	}
	return uninstallPlan{dir: dir, marker: marker, unexpected: unexpected}, nil
}

func uninstallWithProgress(plan uninstallPlan, removeSettings bool) (string, error) {
	updateProgress(5, "Preparando a desinstalação...")
	stopRunningDualCenter()

	updateProgress(25, "Restaurando a configuração da Xbox Game Bar...")
	warning := ""
	if err := restoreGameBarFromSettings(); err != nil {
		warning = "Não foi possível restaurar automaticamente a configuração anterior da Game Bar: " + err.Error()
	}

	updateProgress(45, "Removendo integrações do Windows...")
	removeStartupValue()
	removeRegTree(0x80000002, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, "DualCenter")
	_ = os.Remove(startMenuShortcut())
	_ = os.Remove(desktopShortcut())

	if removeSettings {
		updateProgress(65, "Removendo configurações e logs...")
		_ = os.RemoveAll(localAppDataDir())
	}

	updateProgress(85, "Agendando a remoção dos arquivos...")
	if err := removeOwnedFilesAfterExit(plan.dir, plan.marker, os.Getpid()); err != nil {
		return warning, fmt.Errorf("não foi possível agendar a remoção dos executáveis: %w", err)
	}
	updateProgress(100, "DualCenter removido com sucesso.")
	return warning, nil
}

func uninstallQuiet() {
	plan, err := prepareUninstall()
	if err != nil {
		return
	}
	_, _ = uninstallWithProgress(plan, false)
}
