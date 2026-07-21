// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"dualcenter/internal/version"
	"embed"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed payload/DualCenter.exe
var payloadFS embed.FS

const (
	productName = "DualCenter"

	regSZ        = 1
	regDWORD     = 4
	keyAllAccess = 0xF003F

	mbOK           = 0x00000000
	mbIconInfo     = 0x00000040
	mbIconError    = 0x00000010
	createNoWindow = 0x08000000

	smCXScreen = 0
	smCYScreen = 1

	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsTabStop      = 0x00010000
	wsBorder       = 0x00800000
	wsClipSiblings = 0x04000000

	esAutoHScroll   = 0x0080
	bsPushButton    = 0x00000000
	bsDefPushButton = 0x00000001
	bsAutoCheckbox  = 0x00000003
	ssLeft          = 0x00000000

	wmCreate         = 0x0001
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetIcon        = 0x0080
	wmSetFont        = 0x0030
	wmCtlColorStatic = 0x0138
	wmAppInstallDone = 0x8001
	wmAppProgress    = 0x8002
	wmUser           = 0x0400
	pbmSetRange32    = wmUser + 6
	pbmSetPos        = wmUser + 2
	pbmSetBarColor   = wmUser + 9
	bmSetCheck       = 0x00F1
	bmGetCheck       = 0x00F0
	bstChecked       = 1

	swHide = 0
	swShow = 5

	imageIcon    = 1
	headerHeight = 88

	installerWindowWidth  = 660
	installerWindowHeight = 400

	dwmwaUseImmersiveDarkMode = 20
	dwmwaCaptionColor         = 35
	dwmwaTextColor            = 36

	idEditDir  = 1001
	idBrowse   = 1002
	idDesktop  = 1003
	idLaunch   = 1004
	idInstall  = 1005
	idCancel   = 1006
	idProgress = 1007
	idStatus   = 1008
	idGameBar  = 1009
	idSettings = 1010

	colorWindow     = 5
	transparentMode = 1
	diNormal        = 0x0003
	fwNormal        = 400

	bifReturnOnlyFSDirs = 0x0001
	bifEditBox          = 0x0010
	bifNewDialogStyle   = 0x0040
	bifUseNewUI         = bifEditBox | bifNewDialogStyle

	pageSetup    = 0
	pageProgress = 1

	modeInstall   = 0
	modeUninstall = 1
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type rect struct {
	left, top, right, bottom int32
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type initCommonControlsEx struct {
	dwSize uint32
	dwICC  uint32
}

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

var (
	windowIcon uintptr
	headerIcon uintptr

	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pUpdateWindow                  = user32.NewProc("UpdateWindow")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pSetWindowTextW                = user32.NewProc("SetWindowTextW")
	pGetWindowTextW                = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pLoadIconW                     = user32.NewProc("LoadIconW")
	pLoadImageW                    = user32.NewProc("LoadImageW")
	pSetFocus                      = user32.NewProc("SetFocus")
	pFindWindowW                   = user32.NewProc("FindWindowW")
	pGetWindowThreadProcessId      = user32.NewProc("GetWindowThreadProcessId")
	pGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	pGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")

	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pFillRect         = user32.NewProc("FillRect")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pTextOutW         = gdi32.NewProc("TextOutW")
	pRoundRect        = gdi32.NewProc("RoundRect")
	pDrawIconEx       = user32.NewProc("DrawIconEx")

	pGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")
	pOpenProcess           = kernel32.NewProc("OpenProcess")
	pTerminateProcess      = kernel32.NewProc("TerminateProcess")
	pWaitForSingleObject   = kernel32.NewProc("WaitForSingleObject")
	pCloseHandle           = kernel32.NewProc("CloseHandle")
	pRegCreateKeyExW       = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW        = advapi32.NewProc("RegSetValueExW")
	pRegDeleteTreeW        = advapi32.NewProc("RegDeleteTreeW")
	pRegDeleteValueW       = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey           = advapi32.NewProc("RegCloseKey")
	pSHBrowseForFolderW    = shell32.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW  = shell32.NewProc("SHGetPathFromIDListW")
	pSHChangeNotify        = shell32.NewProc("SHChangeNotify")
	pCoTaskMemFree         = ole32.NewProc("CoTaskMemFree")
	pCoInitializeEx        = ole32.NewProc("CoInitializeEx")
	pCoUninitialize        = ole32.NewProc("CoUninitialize")
	pInitCommonControlsEx  = comctl32.NewProc("InitCommonControlsEx")
	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	hwndMain        uintptr
	hEditDir        uintptr
	hBrowse         uintptr
	hDesktop        uintptr
	hLaunch         uintptr
	hGameBar        uintptr
	hRemoveSettings uintptr
	hInstall        uintptr
	hCancel         uintptr
	hProgress       uintptr
	hStatus         uintptr
	fontNormal      uintptr
	fontSmall       uintptr
	fontHeader      uintptr
	fontBrand       uintptr
	fontButton      uintptr
	bgBrush         uintptr
	stateInstalling bool
	stateDone       bool
	statePage       = pageSetup
	windowMode      = modeInstall
	installErrMu    sync.Mutex
	lastInstallErr  string
	lastWarning     string
	uninstallState  uninstallPlan

	progressMu      sync.Mutex
	progressPercent int
	progressStatus        = "Pronto para instalar."
	setupDPI        int32 = 96
)

func enableInstallerDPIAwareness() {
	// Per Monitor v2 evita que o Windows estique o instalador em 4K,
	// mantendo texto e controles nítidos em escala alta.
	pSetProcessDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if dpi, _, _ := pGetDpiForSystem.Call(); dpi >= 96 && dpi <= 384 {
		setupDPI = int32(dpi)
	}
}

func scale(v int32) int32 {
	return int32((int64(v)*int64(setupDPI) + 48) / 96)
}

func scaleFont(height int32) int32 {
	if height >= 0 {
		return scale(height)
	}
	return -scale(-height)
}

func centeredWindowRect(w, h int32) (int32, int32, int32, int32) {
	sw, _, _ := pGetSystemMetrics.Call(smCXScreen)
	sh, _, _ := pGetSystemMetrics.Call(smCYScreen)
	x := (int32(sw) - w) / 2
	y := (int32(sh) - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y, w, h
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func utf16String(buf []uint16) string {
	for i, c := range buf {
		if c == 0 {
			return syscall.UTF16ToString(buf[:i])
		}
	}
	return syscall.UTF16ToString(buf)
}

func rgb(r, g, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

func styleInstallerTitleBar(hwnd uintptr) {
	enabled := int32(1)
	captionColor := uint32(rgb(8, 17, 36))
	textColor := uint32(rgb(255, 255, 255))
	pDwmSetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	pDwmSetWindowAttribute.Call(hwnd, dwmwaCaptionColor, uintptr(unsafe.Pointer(&captionColor)), unsafe.Sizeof(captionColor))
	pDwmSetWindowAttribute.Call(hwnd, dwmwaTextColor, uintptr(unsafe.Pointer(&textColor)), unsafe.Sizeof(textColor))
}

func message(title, body string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(body))), uintptr(unsafe.Pointer(utf16Ptr(title))), flags)
	return int(r)
}

func hiddenCommandDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = os.TempDir()
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func productVersion() string { return version.Current.Version }
