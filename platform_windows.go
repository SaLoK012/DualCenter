// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"embed"
	"time"
	"unsafe"
)

//go:embed assets/controller_ui_*.png assets/icon_*.png
var embeddedAssets embed.FS

const (
	appName                    = "DualCenter"
	windowClassName            = "DualCenterOverlayWindow"
	mutexName                  = `Local\DualCenter.SingleInstance`
	startupValueName           = "DualCenter"
	fullscreenUnavailableTitle = "Overlay indisponível"
	fullscreenUnavailableBody  = "Use Tela cheia sem borda ou Janela para abrir o menu."
	// A superfície transparente precisa ultrapassar o fim do desfoque para a
	// sombra não encostar nos limites da janela e formar recortes nos cantos.
	fullscreenWarningShadowPadding int32 = 16
	fullscreenWarningShadowExtent        = 14.0
	psDoublePressWindow                  = 450 * time.Millisecond
	psDuplicateGuard                     = 35 * time.Millisecond
	psReleaseStableGap                   = 12 * time.Millisecond
	selectionTransitionDuration          = 120 * time.Millisecond
	selectionTransitionFrames            = 6

	WM_DESTROY              = 0x0002
	WM_CLOSE                = 0x0010
	WM_PAINT                = 0x000F
	WM_SETTINGCHANGE        = 0x001A
	WM_ERASEBKGND           = 0x0014
	WM_SETCURSOR            = 0x0020
	WM_MOUSEACTIVATE        = 0x0021
	WM_DISPLAYCHANGE        = 0x007E
	WM_INPUT_DEVICE_CHANGE  = 0x00FE
	WM_INPUT                = 0x00FF
	WM_DPICHANGED           = 0x02E0
	WM_TIMER                = 0x0113
	WM_SETICON              = 0x0080
	WM_APP_ADD_GAME         = 0x8001
	WM_APP_OPEN_GAME_DIALOG = 0x8002
	WM_APP_TASKBAR_ICON     = 0x8003
	WM_APP_EXIT             = 0x8004
	GIDC_REMOVAL            = 2
	RIM_TYPEHID             = 2
	RID_INPUT               = 0x10000003
	RIDI_DEVICENAME         = 0x20000007
	RIDI_DEVICEINFO         = 0x2000000B
	RIDEV_INPUTSINK         = 0x00000100
	RIDEV_DEVNOTIFY         = 0x00002000

	WS_POPUP                                  = 0x80000000
	WS_EX_TRANSPARENT                         = 0x20
	WS_EX_TOOLWINDOW                          = 0x80
	WS_EX_TOPMOST                             = 0x8
	WS_EX_LAYERED                             = 0x80000
	WS_EX_NOACTIVATE                          = 0x08000000
	WS_EX_APPWINDOW                           = 0x00040000
	GWL_EXSTYLE                       uintptr = ^uintptr(19)
	CS_HREDRAW                                = 0x2
	CS_VREDRAW                                = 0x1
	SW_HIDE                                   = 0
	SW_SHOW                                   = 5
	SW_RESTORE                                = 9
	SW_SHOWNOACTIVATE                         = 4
	SWP_NOSIZE                                = 0x1
	SWP_NOMOVE                                = 0x2
	SWP_NOACTIVATE                            = 0x10
	SWP_FRAMECHANGED                          = 0x20
	SWP_SHOWWINDOW                            = 0x40
	HWND_TOPMOST                              = ^uintptr(0)
	ULW_ALPHA                                 = 0x2
	DT_LEFT                                   = 0
	DT_CENTER                                 = 1
	DT_RIGHT                                  = 2
	DT_VCENTER                                = 4
	DT_SINGLELINE                             = 0x20
	DT_CALCRECT                               = 0x400
	DT_END_ELLIPSIS                           = 0x8000
	TRANSPARENT                               = 1
	PS_SOLID                                  = 0
	FW_NORMAL                                 = 400
	FW_MEDIUM                                 = 500
	FW_SEMIBOLD                               = 600
	AC_SRC_OVER                               = 0
	AC_SRC_ALPHA                              = 1
	BI_RGB                                    = 0
	DIB_RGB_COLORS                            = 0
	MONITOR_DEFAULTTONEAREST                  = 2
	COINIT_APARTMENTTHREADED                  = 0x2
	CLSCTX_ALL                                = 0x17
	STGM_READ                                 = 0
	DEVICE_STATE_ACTIVE                       = 0x1
	E_RENDER                                  = 0
	REG_SZ                                    = 1
	REG_DWORD                                 = 4
	KEY_SET_VALUE                             = 0x0002
	KEY_QUERY_VALUE                           = 0x0001
	ERROR_FILE_NOT_FOUND                      = 2
	ERROR_ALREADY_EXISTS                      = 183
	DISPLAY_RELAYOUT_TIMER_ID                 = 2
	CREATE_NO_WINDOW                          = 0x08000000
	GENERIC_READ                              = 0x80000000
	GENERIC_WRITE                             = 0x40000000
	FILE_SHARE_READ                           = 1
	FILE_SHARE_WRITE                          = 2
	OPEN_EXISTING                             = 3
	FILE_ATTRIBUTE_NORMAL                     = 0x80
	INVALID_HANDLE_VALUE                      = ^uintptr(0)
	TH32CS_SNAPPROCESS                        = 0x00000002
	PROCESS_QUERY_LIMITED_INFORMATION         = 0x1000
	PROCESS_TERMINATE                         = 0x0001
	SHGFI_ICON                                = 0x000000100
	SHGFI_LARGEICON                           = 0x000000000
	SHGFI_SYSICONINDEX                        = 0x000004000
	SHIL_EXTRALARGE                           = 0x2
	SHIL_JUMBO                                = 0x4
	FOS_FORCEFILESYSTEM                       = 0x00000040
	FOS_PATHMUSTEXIST                         = 0x00000800
	FOS_FILEMUSTEXIST                         = 0x00001000
	FOS_DONTADDTORECENT                       = 0x02000000
	SIGDN_FILESYSPATH                         = 0x80058000
	ILD_TRANSPARENT                           = 0x1
	DI_NORMAL                                 = 0x0003
	MA_NOACTIVATE                             = 3
	QUNS_RUNNING_D3D_FULL_SCREEN              = 3
	CURSOR_SHOWING                            = 0x00000001

	tabVolume   = 0
	tabEnergy   = 1
	tabModeGame = 2
	tabSettings = 3
	tabBattery  = 4
	tabGames    = 5
	tabCount    = 6

	overlayHidden  = 0
	overlayBattery = 1
	overlayMenu    = 2
	overlayMessage = 3

	controllerModelUnknown  = 0
	controllerModelStandard = 1
	controllerModelEdge     = 2
)

type (
	point      struct{ X, Y int32 }
	size       struct{ CX, CY int32 }
	rect       struct{ Left, Top, Right, Bottom int32 }
	cursorInfo struct {
		CbSize         uint32
		Flags          uint32
		Cursor         uintptr
		ScreenPosition point
	}
	msg struct {
		Hwnd           uintptr
		Message        uint32
		_              uint32
		WParam, LParam uintptr
		Time           uint32
		Pt             point
		LPrivate       uint32
	}
)

type wndClassEx struct {
	CbSize, Style                            uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground uintptr
	LpszMenuName, LpszClassName              *uint16
	HIconSm                                  uintptr
}
type paintStruct struct {
	Hdc                  uintptr
	FErase               int32
	RcPaint              rect
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}
type rawInputDevice struct {
	UsagePage, Usage uint16
	Flags            uint32
	Target           uintptr
}
type rawInputHeader struct {
	Type, Size     uint32
	Device, WParam uintptr
}
type monitorInfo struct {
	CbSize        uint32
	Monitor, Work rect
	Flags         uint32
}
type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}
type (
	blendFunction struct{ BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte }
	guid          struct {
		Data1        uint32
		Data2, Data3 uint16
		Data4        [8]byte
	}
)

type propertyKey struct {
	Fmtid guid
	Pid   uint32
}
type propVariant struct {
	Vt  uint16
	_   [6]byte
	Ptr unsafe.Pointer
	_2  [8]byte
}
type audioDevice struct{ ID, Name string }

type comdlgFilterSpec struct {
	Name *uint16
	Spec *uint16
}

type shFileInfo struct {
	HIcon       uintptr
	IIcon       int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}
