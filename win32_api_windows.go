// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	hid      = syscall.NewLazyDLL("hid.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW             = user32.NewProc("RegisterClassExW")
	pCreateWindowExW              = user32.NewProc("CreateWindowExW")
	pDefWindowProcW               = user32.NewProc("DefWindowProcW")
	pGetMessageW                  = user32.NewProc("GetMessageW")
	pTranslateMessage             = user32.NewProc("TranslateMessage")
	pDispatchMessageW             = user32.NewProc("DispatchMessageW")
	pPostQuitMessage              = user32.NewProc("PostQuitMessage")
	pRegisterRawInputDevices      = user32.NewProc("RegisterRawInputDevices")
	pGetRawInputData              = user32.NewProc("GetRawInputData")
	pGetRawInputDeviceInfoW       = user32.NewProc("GetRawInputDeviceInfoW")
	pSetTimer                     = user32.NewProc("SetTimer")
	pKillTimer                    = user32.NewProc("KillTimer")
	pShowWindow                   = user32.NewProc("ShowWindow")
	pSetWindowPos                 = user32.NewProc("SetWindowPos")
	pUpdateLayeredWindow          = user32.NewProc("UpdateLayeredWindow")
	pInvalidateRect               = user32.NewProc("InvalidateRect")
	pGetDC                        = user32.NewProc("GetDC")
	pReleaseDC                    = user32.NewProc("ReleaseDC")
	pBeginPaint                   = user32.NewProc("BeginPaint")
	pEndPaint                     = user32.NewProc("EndPaint")
	pGetClientRect                = user32.NewProc("GetClientRect")
	pGetWindowRect                = user32.NewProc("GetWindowRect")
	pGetCursorPos                 = user32.NewProc("GetCursorPos")
	pClipCursor                   = user32.NewProc("ClipCursor")
	pBlockInput                   = user32.NewProc("BlockInput")
	pFillRect                     = user32.NewProc("FillRect")
	pDrawTextW                    = user32.NewProc("DrawTextW")
	pGetForegroundWindow          = user32.NewProc("GetForegroundWindow")
	pSetForegroundWindow          = user32.NewProc("SetForegroundWindow")
	pIsIconic                     = user32.NewProc("IsIconic")
	pBringWindowToTop             = user32.NewProc("BringWindowToTop")
	pSetFocus                     = user32.NewProc("SetFocus")
	pAttachThreadInput            = user32.NewProc("AttachThreadInput")
	pIsWindow                     = user32.NewProc("IsWindow")
	pGetCapture                   = user32.NewProc("GetCapture")
	pSetCapture                   = user32.NewProc("SetCapture")
	pReleaseCapture               = user32.NewProc("ReleaseCapture")
	pGetWindowLongPtrW            = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW            = user32.NewProc("SetWindowLongPtrW")
	pShowCursor                   = user32.NewProc("ShowCursor")
	pGetCursorInfo                = user32.NewProc("GetCursorInfo")
	pKeybdEvent                   = user32.NewProc("keybd_event")
	pSetCursor                    = user32.NewProc("SetCursor")
	pSetCursorPos                 = user32.NewProc("SetCursorPos")
	pMonitorFromWindow            = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW              = user32.NewProc("GetMonitorInfoW")
	pGetSystemMetrics             = user32.NewProc("GetSystemMetrics")
	pMessageBoxW                  = user32.NewProc("MessageBoxW")
	pPostMessageW                 = user32.NewProc("PostMessageW")
	pSendMessageW                 = user32.NewProc("SendMessageW")
	pFindWindowW                  = user32.NewProc("FindWindowW")
	pLoadIconW                    = user32.NewProc("LoadIconW")
	pGetWindowThreadProcessId     = user32.NewProc("GetWindowThreadProcessId")
	pDrawIconEx                   = user32.NewProc("DrawIconEx")
	pDestroyIcon                  = user32.NewProc("DestroyIcon")
	pGetCurrentThreadId           = kernel32.NewProc("GetCurrentThreadId")
	pSHGetFileInfoW               = shell32.NewProc("SHGetFileInfoW")
	pSHGetImageList               = shell32.NewProc("SHGetImageList")
	pSHQueryUserNotificationState = shell32.NewProc("SHQueryUserNotificationState")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	pGetStockObject     = gdi32.NewProc("GetStockObject")
	pCreatePen          = gdi32.NewProc("CreatePen")
	pRoundRect          = gdi32.NewProc("RoundRect")
	pMoveToEx           = gdi32.NewProc("MoveToEx")
	pLineTo             = gdi32.NewProc("LineTo")
	pCreateFontW        = gdi32.NewProc("CreateFontW")
	pSetTextColor       = gdi32.NewProc("SetTextColor")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")

	pGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW               = kernel32.NewProc("CreateMutexW")
	pCreateFileW                = kernel32.NewProc("CreateFileW")
	pCloseHandle                = kernel32.NewProc("CloseHandle")
	pCreateToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	pProcess32FirstW            = kernel32.NewProc("Process32FirstW")
	pProcess32NextW             = kernel32.NewProc("Process32NextW")
	pOpenProcess                = kernel32.NewProc("OpenProcess")
	pQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	pTerminateProcess           = kernel32.NewProc("TerminateProcess")
	pHidDGetFeature             = hid.NewProc("HidD_GetFeature")
	pCoInitializeEx             = ole32.NewProc("CoInitializeEx")
	pCoUninitialize             = ole32.NewProc("CoUninitialize")
	pCoCreateInstance           = ole32.NewProc("CoCreateInstance")
	pCoTaskMemFree              = ole32.NewProc("CoTaskMemFree")
	pPropVariantClear           = ole32.NewProc("PropVariantClear")
	pRegCreateKeyExW            = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW             = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW            = advapi32.NewProc("RegDeleteValueW")
	pRegQueryValueExW           = advapi32.NewProc("RegQueryValueExW")
	pRegCloseKey                = advapi32.NewProc("RegCloseKey")
)

func utf16Ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func succeeded(hr uintptr) bool { return int32(hr) >= 0 }
func vcall(obj unsafe.Pointer, index int, args ...uintptr) uintptr {
	if obj == nil {
		return 0
	}
	vt := *(*unsafe.Pointer)(obj)
	if vt == nil {
		return 0
	}
	fn := *(*uintptr)(unsafe.Add(vt, uintptr(index)*unsafe.Sizeof(uintptr(0))))
	a := append([]uintptr{uintptr(obj)}, args...)
	r, _, _ := syscall.SyscallN(fn, a...)
	return r
}
