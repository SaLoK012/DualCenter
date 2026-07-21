// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"dualcenter/internal/version"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func chooseFolder(owner uintptr) string {
	pCoInitializeEx.Call(0, 2)
	defer pCoUninitialize.Call()
	display := make([]uint16, 260)
	bi := browseInfo{
		hwndOwner:      owner,
		pszDisplayName: &display[0],
		lpszTitle:      utf16Ptr("Escolha onde instalar o DualCenter"),
		ulFlags:        bifReturnOnlyFSDirs | bifUseNewUI,
	}
	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer pCoTaskMemFree.Call(pidl)
	path := make([]uint16, 32768)
	r, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if r == 0 {
		return ""
	}
	selected := utf16String(path)
	if selected == "" {
		return ""
	}
	if strings.EqualFold(filepath.Base(selected), productName) {
		return selected
	}
	return filepath.Join(selected, productName)
}

func drawText(hdc uintptr, x, y int32, text string, font uintptr, color uintptr) {
	if font != 0 {
		old, _, _ := pSelectObject.Call(hdc, font)
		defer pSelectObject.Call(hdc, old)
	}
	pSetBkMode.Call(hdc, transparentMode)
	pSetTextColor.Call(hdc, color)
	t, err := syscall.UTF16FromString(text)
	if err != nil {
		return
	}
	pTextOutW.Call(hdc, uintptr(scale(x)), uintptr(scale(y)), uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1))
}

func fillRect(hdc uintptr, r rect, color uintptr) {
	brush, _, _ := pCreateSolidBrush.Call(color)
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	pDeleteObject.Call(brush)
}

func drawInstallerWindow(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	fillRect(hdc, rc, rgb(255, 255, 255))

	// Cabeçalho compacto e alinhado à barra de título escura.
	headerH := scale(headerHeight)
	for i := int32(0); i < headerH; i++ {
		r := rect{0, i, rc.right, i + 1}
		progress := int(i * 255 / headerH)
		c := rgb(byte(8+progress*5/255), byte(17+progress*14/255), byte(36+progress*23/255))
		fillRect(hdc, r, c)
	}
	fillRect(hdc, rect{0, headerH - scale(2), rc.right, headerH}, rgb(37, 99, 235))

	// Ícone e identidade visual oficial.
	if headerIcon != 0 {
		pDrawIconEx.Call(hdc, uintptr(scale(28)), uintptr(scale(17)), headerIcon, uintptr(scale(54)), uintptr(scale(54)), 0, 0, diNormal)
	} else if windowIcon != 0 {
		pDrawIconEx.Call(hdc, uintptr(scale(28)), uintptr(scale(17)), windowIcon, uintptr(scale(54)), uintptr(scale(54)), 0, 0, diNormal)
	} else {
		logoBrush, _, _ := pCreateSolidBrush.Call(rgb(37, 99, 235))
		oldBrush, _, _ := pSelectObject.Call(hdc, logoBrush)
		pRoundRect.Call(hdc, uintptr(scale(28)), uintptr(scale(17)), uintptr(scale(82)), uintptr(scale(71)), uintptr(scale(18)), uintptr(scale(18)))
		pSelectObject.Call(hdc, oldBrush)
		pDeleteObject.Call(logoBrush)
		drawText(hdc, 39, 31, "D", fontHeader, rgb(255, 255, 255))
		drawText(hdc, 58, 31, "C", fontHeader, rgb(219, 234, 254))
	}

	drawText(hdc, 100, 17, "DualCenter", fontBrand, rgb(255, 255, 255))
	if windowMode == modeInstall {
		drawText(hdc, 102, 51, "Setup v"+version.Current.Version+"  •  Windows 10/11  •  64 bits", fontSmall, rgb(203, 213, 225))
	} else {
		drawText(hdc, 102, 51, "Desinstalador oficial  •  v"+version.Current.Version+"  •  64 bits", fontSmall, rgb(203, 213, 225))
	}

	// Rodapé separado do conteúdo; a área principal fica limpa, sem moldura.
	fillRect(hdc, rect{0, scale(300), rc.right, rc.bottom}, rgb(246, 248, 252))
	fillRect(hdc, rect{0, scale(300), rc.right, scale(301)}, rgb(226, 232, 240))

	if statePage == pageSetup && windowMode == modeInstall {
		drawText(hdc, 32, 108, "Instale o DualCenter", fontHeader, rgb(15, 23, 42))
		drawText(hdc, 32, 134, "Escolha o destino e confirme suas preferências.", fontSmall, rgb(71, 85, 105))
		drawText(hdc, 32, 160, "Pasta de instalação", fontButton, rgb(51, 65, 85))
	} else if statePage == pageSetup {
		drawText(hdc, 32, 108, "Desinstalar o DualCenter", fontHeader, rgb(15, 23, 42))
		drawText(hdc, 32, 134, "Remova o aplicativo com segurança deste computador.", fontSmall, rgb(71, 85, 105))
		drawText(hdc, 32, 176, "O desinstalador irá:", fontButton, rgb(51, 65, 85))
		drawText(hdc, 32, 204, "• Remover o aplicativo e seus atalhos", fontSmall, rgb(71, 85, 105))
		drawText(hdc, 32, 228, "• Restaurar a configuração anterior da Xbox Game Bar", fontSmall, rgb(71, 85, 105))
	} else if windowMode == modeInstall {
		drawText(hdc, 32, 112, "Instalando o DualCenter", fontHeader, rgb(15, 23, 42))
		drawText(hdc, 32, 140, "Aguarde enquanto preparamos tudo para você.", fontSmall, rgb(71, 85, 105))
		drawText(hdc, 32, 190, "Progresso", fontButton, rgb(51, 65, 85))
	} else {
		drawText(hdc, 32, 112, "Desinstalando o DualCenter", fontHeader, rgb(15, 23, 42))
		drawText(hdc, 32, 140, "Aguarde enquanto removemos o aplicativo com segurança.", fontSmall, rgb(71, 85, 105))
		drawText(hdc, 32, 190, "Progresso", fontButton, rgb(51, 65, 85))
	}
	if windowMode == modeInstall {
		drawText(hdc, 32, 326, "Instalação oficial, segura e local", fontSmall, rgb(100, 116, 139))
	} else {
		drawText(hdc, 32, 326, "Remoção protegida dos arquivos do DualCenter", fontSmall, rgb(100, 116, 139))
	}
}

func onCreate(hwnd uintptr) {
	hwndMain = hwnd
	fontNormal = createFont(-15, fwNormal, "Segoe UI")
	fontSmall = createFont(-13, fwNormal, "Segoe UI")
	fontHeader = createFont(-19, fwNormal, "Segoe UI")
	fontBrand = createFont(-26, fwNormal, "Segoe UI")
	fontButton = createFont(-14, fwNormal, "Segoe UI")
	bgBrush, _, _ = pCreateSolidBrush.Call(rgb(255, 255, 255))

	if windowMode == modeInstall {
		hEditDir = createControl("Edit", installDir(), wsChild|wsVisible|wsBorder|wsTabStop|esAutoHScroll, 0, 32, 182, 456, 30, idEditDir)
		hBrowse = createControl("Button", "Selecionar", wsChild|wsVisible|wsTabStop|bsPushButton, 0, 500, 180, 128, 34, idBrowse)
		hDesktop = createControl("Button", "Criar atalho na Área de Trabalho", wsChild|wsVisible|wsTabStop|bsAutoCheckbox, 0, 32, 220, 300, 22, idDesktop)
		hLaunch = createControl("Button", "Abrir o DualCenter ao finalizar", wsChild|wsVisible|wsTabStop|bsAutoCheckbox, 0, 32, 244, 300, 22, idLaunch)
		hGameBar = createControl("Button", "Manter a Xbox Game Bar ativada", wsChild|wsVisible|wsTabStop|bsAutoCheckbox, 0, 32, 268, 300, 22, idGameBar)
		setChecked(hDesktop, true)
		setChecked(hLaunch, true)
		setChecked(hGameBar, true)
	} else {
		hRemoveSettings = createControl("Button", "Apagar também configurações e logs", wsChild|wsVisible|wsTabStop|bsAutoCheckbox, 0, 32, 264, 330, 24, idSettings)
	}
	hProgress = createControl("msctls_progress32", "", wsChild|wsVisible|wsClipSiblings, 0, 32, 216, 596, 20, idProgress)
	pSendMessageW.Call(hProgress, pbmSetRange32, 0, 100)
	pSendMessageW.Call(hProgress, pbmSetBarColor, 0, rgb(37, 99, 235))
	hStatus = createControl("Static", "Pronto.", wsChild|wsVisible|ssLeft, 0, 32, 250, 596, 26, idStatus)
	if fontSmall != 0 {
		pSendMessageW.Call(hStatus, wmSetFont, fontSmall, 1)
	}
	hCancel = createControl("Button", "Cancelar", wsChild|wsVisible|wsTabStop|bsPushButton, 0, 384, 314, 108, 36, idCancel)
	hInstall = createControl("Button", "Instalar agora", wsChild|wsVisible|wsTabStop|bsDefPushButton, 0, 504, 314, 108, 36, idInstall)
	if fontButton != 0 {
		if hBrowse != 0 {
			pSendMessageW.Call(hBrowse, wmSetFont, fontButton, 1)
		}
		for _, action := range []uintptr{hDesktop, hLaunch, hGameBar, hRemoveSettings} {
			if action != 0 {
				pSendMessageW.Call(action, wmSetFont, fontButton, 1)
			}
		}
		pSendMessageW.Call(hCancel, wmSetFont, fontButton, 1)
		pSendMessageW.Call(hInstall, wmSetFont, fontButton, 1)
	}
	showSetupPage()
	pSetFocus.Call(hInstall)
}

func startInstall() {
	if stateDone {
		pDestroyWindow.Call(hwndMain)
		return
	}
	if stateInstalling {
		return
	}
	dir := strings.TrimSpace(getWindowText(hEditDir))
	normalized, err := normalizeInstallDir(dir)
	if err != nil {
		message("DualCenter Setup", "Escolha uma pasta de instalação segura.\n\n"+err.Error(), mbOK|mbIconError)
		return
	}
	setWindowText(hEditDir, normalized)
	opts := installOptions{
		dir:             normalized,
		createDesktop:   checked(hDesktop),
		launchAfterDone: checked(hLaunch),
		gameBarEnabled:  checked(hGameBar),
	}
	stateInstalling = true
	setOperationWarning("")
	showProgressPage()
	updateProgress(0, "Iniciando instalação...")

	go func() {
		err := installWithProgress(opts)
		if err != nil {
			setInstallError(err.Error())
		} else {
			setInstallError("")
		}
		if err == nil && opts.launchAfterDone {
			if launchErr := launchInstalledApp(opts.dir); launchErr != nil {
				setInstallError("Instalação concluída, mas não foi possível abrir o DualCenter automaticamente: " + launchErr.Error())
			}
		}
		pPostMessageW.Call(hwndMain, wmAppInstallDone, 0, 0)
	}()
}

func startUninstall() {
	if stateDone {
		pDestroyWindow.Call(hwndMain)
		return
	}
	if stateInstalling {
		return
	}
	stateInstalling = true
	setInstallError("")
	setOperationWarning("")
	showProgressPage()
	updateProgress(0, "Iniciando desinstalação...")
	removeSettings := checked(hRemoveSettings)

	go func() {
		warning, err := uninstallWithProgress(uninstallState, removeSettings)
		setOperationWarning(warning)
		if err != nil {
			setInstallError(err.Error())
		} else {
			setInstallError("")
		}
		pPostMessageW.Call(hwndMain, wmAppInstallDone, 0, 0)
	}()
}

func completeInstall() {
	stateInstalling = false
	errMessage := installError()
	if errMessage != "" {
		updateProgress(0, "A instalação falhou.")
		showSetupPage()
		message("Falha na instalação", "Não foi possível instalar o DualCenter.\n\n"+errMessage, mbOK|mbIconError)
		return
	}
	stateDone = true
	updateProgress(100, "Instalação concluída. Clique em Concluir para fechar.")
	setWindowText(hInstall, "Concluir")
	showControl(hInstall, true)
	pSetFocus.Call(hInstall)
}

func completeUninstall() {
	stateInstalling = false
	errMessage := installError()
	if errMessage != "" {
		updateProgress(0, "A desinstalação não pôde ser concluída.")
		showSetupPage()
		message("Falha na desinstalação", "Não foi possível remover o DualCenter.\n\n"+errMessage, mbOK|mbIconError)
		return
	}
	stateDone = true
	status := "DualCenter removido. Clique em Concluir para fechar."
	if len(uninstallState.unexpected) > 0 {
		status = "DualCenter removido. Outros arquivos da pasta foram preservados."
	}
	if warning := operationWarning(); warning != "" {
		status = "DualCenter removido com um aviso sobre a Xbox Game Bar."
		message("Desinstalação concluída com aviso", warning, mbOK|mbIconInfo)
	}
	updateProgress(100, status)
	setWindowText(hInstall, "Concluir")
	showControl(hInstall, true)
	pSetFocus.Call(hInstall)
}

func wndProc(hwnd uintptr, msgID uint32, wParam, lParam uintptr) uintptr {
	switch msgID {
	case wmCreate:
		onCreate(hwnd)
		return 0
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		drawInstallerWindow(hwnd)
		return 0
	case wmCtlColorStatic:
		pSetBkMode.Call(wParam, transparentMode)
		pSetTextColor.Call(wParam, rgb(51, 65, 85))
		return bgBrush
	case wmCommand:
		id := uint16(wParam & 0xffff)
		switch id {
		case idBrowse:
			selected := chooseFolder(hwnd)
			if selected != "" {
				setWindowText(hEditDir, selected)
			}
		case idInstall:
			if windowMode == modeInstall {
				startInstall()
			} else {
				startUninstall()
			}
		case idCancel:
			if !stateInstalling {
				pDestroyWindow.Call(hwnd)
			}
		}
		return 0
	case wmAppProgress:
		applyProgress()
		return 0
	case wmAppInstallDone:
		applyProgress()
		if windowMode == modeInstall {
			completeInstall()
		} else {
			completeUninstall()
		}
		return 0
	case wmClose:
		if stateInstalling {
			if windowMode == modeInstall {
				message("DualCenter Setup", "Aguarde a instalação terminar antes de fechar.", mbOK|mbIconInfo)
			} else {
				message("Desinstalar DualCenter", "Aguarde a desinstalação terminar antes de fechar.", mbOK|mbIconInfo)
			}
			return 0
		}
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		for _, h := range []uintptr{fontNormal, fontSmall, fontHeader, fontBrand, fontButton, bgBrush} {
			if h != 0 {
				pDeleteObject.Call(h)
			}
		}
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msgID), wParam, lParam)
	return r
}

func runInstallerGUI() int {
	windowMode = modeInstall
	return runWindow()
}

func runUninstallerGUI() int {
	plan, err := prepareUninstall()
	if err != nil {
		message("Desinstalação protegida", "O DualCenter não removeu nenhum arquivo.\n\n"+err.Error(), mbOK|mbIconError)
		return 1
	}
	uninstallState = plan
	windowMode = modeUninstall
	return runWindow()
}

func runWindow() int {
	runtime.LockOSThread()
	enableInstallerDPIAwareness()
	icc := initCommonControlsEx{dwSize: uint32(unsafe.Sizeof(initCommonControlsEx{})), dwICC: 0x00000020}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	hInst, _, _ := pGetModuleHandleW.Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, 32512) // IDC_ARROW
	icon, _, _ := pLoadImageW.Call(hInst, 1, imageIcon, uintptr(scale(32)), uintptr(scale(32)), 0)
	smallIcon, _, _ := pLoadImageW.Call(hInst, 1, imageIcon, uintptr(scale(16)), uintptr(scale(16)), 0)
	largeIcon, _, _ := pLoadImageW.Call(hInst, 1, imageIcon, uintptr(scale(64)), uintptr(scale(64)), 0)
	if icon == 0 {
		icon, _, _ = pLoadIconW.Call(hInst, 1)
	}
	if smallIcon == 0 {
		smallIcon = icon
	}
	if largeIcon == 0 {
		largeIcon = icon
	}
	if icon == 0 {
		icon, _, _ = pLoadIconW.Call(0, 32512)
		smallIcon = icon
		largeIcon = icon
	}
	windowIcon = icon
	headerIcon = largeIcon
	className := utf16Ptr("DualCenterProfessionalSetup")
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInst,
		hIcon:         icon,
		hCursor:       cursor,
		hbrBackground: colorWindow + 1,
		lpszClassName: className,
		hIconSm:       smallIcon,
	}
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		message("DualCenter", "Não foi possível registrar a janela.", mbOK|mbIconError)
		return 1
	}
	windowTitle := "DualCenter Setup"
	if windowMode == modeUninstall {
		windowTitle = "Desinstalar DualCenter"
	}
	winX, winY, winW, winH := centeredWindowRect(scale(installerWindowWidth), scale(installerWindowHeight))
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr(windowTitle))),
		wsOverlapped|wsCaption|wsSysMenu|wsMinimizeBox,
		uintptr(winX), uintptr(winY), uintptr(winW), uintptr(winH),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		message("DualCenter", "Não foi possível abrir a janela.", mbOK|mbIconError)
		return 1
	}
	styleInstallerTitleBar(hwnd)
	pSendMessageW.Call(hwnd, wmSetIcon, 1, icon)
	pSendMessageW.Call(hwnd, wmSetIcon, 0, smallIcon)
	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return 0
}
