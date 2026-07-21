//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	taskbarIconID = 1

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	wmContextMenu   = 0x007B
	wmRightButtonUp = 0x0205

	mfString       = 0x0000
	tpmRightButton = 0x0002
	tpmNonotify    = 0x0080
	tpmReturnCmd   = 0x0100
	taskbarCloseID = 1
)

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         guid
	HBalloonIcon     uintptr
}

var (
	pShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	pRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	pDestroyMenu            = user32.NewProc("DestroyMenu")
	taskbarCreatedMessage   uint32
	taskbarIconAdded        bool
)

func taskbarIconData() notifyIconData {
	var data notifyIconData
	data.CbSize = uint32(unsafe.Sizeof(data))
	data.HWnd = mainWindow
	data.UID = taskbarIconID
	data.UFlags = nifMessage | nifIcon | nifTip
	data.UCallbackMessage = WM_APP_TASKBAR_ICON
	hInstance, _, _ := pGetModuleHandleW.Call(0)
	data.HIcon, _, _ = pLoadIconW.Call(hInstance, 1)
	if data.HIcon == 0 {
		data.HIcon, _, _ = pLoadIconW.Call(0, 32512)
	}
	tip, _ := syscall.UTF16FromString("DualCenter")
	copy(data.SzTip[:], tip)
	return data
}

// applyTaskbarIconStyleLocked mantém o overlay fora da lista de janelas comuns
// e controla o ícone residente na área de notificação da barra de tarefas.
func applyTaskbarIconStyleLocked() error {
	if mainWindow == 0 {
		return fmt.Errorf("janela principal ainda não foi criada")
	}
	style, _, _ := pGetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE)
	style &^= WS_EX_APPWINDOW
	style |= WS_EX_TOOLWINDOW
	pSetWindowLongPtrW.Call(mainWindow, GWL_EXSTYLE, style)
	pSetWindowPos.Call(
		mainWindow,
		HWND_TOPMOST,
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_FRAMECHANGED,
	)

	if taskbarCreatedMessage == 0 {
		message, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16Ptr("TaskbarCreated"))))
		taskbarCreatedMessage = uint32(message)
	}
	if state.showTaskbarIcon {
		return ensureTaskbarIconLocked()
	}
	return removeTaskbarIconLocked()
}

func ensureTaskbarIconLocked() error {
	data := taskbarIconData()
	operation := uintptr(nimAdd)
	if taskbarIconAdded {
		operation = nimModify
	}
	ok, _, _ := pShellNotifyIconW.Call(operation, uintptr(unsafe.Pointer(&data)))
	if ok == 0 && taskbarIconAdded {
		// O Explorer pode ter sido reiniciado sem que TaskbarCreated chegasse a
		// tempo. Refazer a inclusão recupera o ícone sem reiniciar o DualCenter.
		taskbarIconAdded = false
		ok, _, _ = pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	}
	if ok == 0 {
		return fmt.Errorf("Shell_NotifyIconW não adicionou o ícone")
	}
	taskbarIconAdded = true
	return nil
}

func removeTaskbarIconLocked() error {
	if !taskbarIconAdded || mainWindow == 0 {
		return nil
	}
	data := taskbarIconData()
	ok, _, _ := pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		return fmt.Errorf("Shell_NotifyIconW não removeu o ícone")
	}
	taskbarIconAdded = false
	return nil
}

func handleTaskbarCreated() {
	state.mu.Lock()
	taskbarIconAdded = false
	if state.showTaskbarIcon {
		if err := ensureTaskbarIconLocked(); err != nil {
			logf("falha ao recuperar o ícone após reiniciar o Explorer: %v", err)
		}
	}
	state.mu.Unlock()
}

func handleTaskbarIconEvent(event uintptr) {
	if uint32(event) != wmRightButtonUp && uint32(event) != wmContextMenu {
		return
	}
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		logf("não foi possível criar o menu do ícone da barra de tarefas")
		return
	}
	defer pDestroyMenu.Call(menu)
	label := utf16Ptr("Fechar")
	if ok, _, _ := pAppendMenuW.Call(menu, mfString, taskbarCloseID, uintptr(unsafe.Pointer(label))); ok == 0 {
		logf("não foi possível adicionar a ação Fechar ao menu da barra de tarefas")
		return
	}
	var cursor point
	if ok, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return
	}
	pSetForegroundWindow.Call(mainWindow)
	selected, _, _ := pTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmNonotify|tpmReturnCmd,
		uintptr(int64(cursor.X)), uintptr(int64(cursor.Y)),
		0,
		mainWindow,
		0,
	)
	pPostMessageW.Call(mainWindow, 0, 0, 0)
	if selected == taskbarCloseID {
		pPostMessageW.Call(mainWindow, WM_APP_EXIT, 0, 0)
	}
}
