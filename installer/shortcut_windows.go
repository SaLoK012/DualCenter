//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	clsctxInprocServer = 0x1
	shcneUpdateItem    = 0x00002000
	shcnfPathW         = 0x0005
)

type comGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidShellLink    = comGUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidShellLinkW     = comGUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidPersistFile    = comGUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
)

func hresultError(action string, result uintptr) error {
	if int32(uint32(result)) < 0 {
		return fmt.Errorf("%s falhou (HRESULT=%#08x)", action, uint32(result))
	}
	return nil
}

func comMethod(object unsafe.Pointer, index int) uintptr {
	vtable := *(**[32]uintptr)(object)
	return vtable[index]
}

func comRelease(object unsafe.Pointer) {
	if object != nil {
		syscall.SyscallN(comMethod(object, 2), uintptr(object))
	}
}

// createShortcutAt usa diretamente IShellLinkW/IPersistFile. Além de evitar um
// processo externo, isso elimina a necessidade de ignorar políticas do
// PowerShell durante uma instalação normal.
func createShortcutAt(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, _ := pCoInitializeEx.Call(0, 2)
	if err := hresultError("CoInitializeEx", result); err != nil {
		return err
	}
	defer pCoUninitialize.Call()

	var shellLink unsafe.Pointer
	result, _, _ = pCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidShellLinkW)),
		uintptr(unsafe.Pointer(&shellLink)),
	)
	if err := hresultError("CoCreateInstance(IShellLinkW)", result); err != nil {
		return err
	}
	defer comRelease(shellLink)

	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	workingPtr, err := syscall.UTF16PtrFromString(filepath.Dir(target))
	if err != nil {
		return err
	}
	descriptionPtr, _ := syscall.UTF16PtrFromString("DualCenter")
	for _, call := range []struct {
		name   string
		method int
		args   []uintptr
	}{
		{name: "IShellLinkW.SetPath", method: 20, args: []uintptr{uintptr(unsafe.Pointer(targetPtr))}},
		{name: "IShellLinkW.SetWorkingDirectory", method: 9, args: []uintptr{uintptr(unsafe.Pointer(workingPtr))}},
		{name: "IShellLinkW.SetDescription", method: 7, args: []uintptr{uintptr(unsafe.Pointer(descriptionPtr))}},
		{name: "IShellLinkW.SetIconLocation", method: 17, args: []uintptr{uintptr(unsafe.Pointer(targetPtr)), 0}},
	} {
		arguments := append([]uintptr{comMethod(shellLink, call.method), uintptr(shellLink)}, call.args...)
		result, _, _ = syscall.SyscallN(arguments[0], arguments[1:]...)
		if err := hresultError(call.name, result); err != nil {
			return err
		}
	}

	var persistFile unsafe.Pointer
	result, _, _ = syscall.SyscallN(
		comMethod(shellLink, 0),
		uintptr(shellLink),
		uintptr(unsafe.Pointer(&iidPersistFile)),
		uintptr(unsafe.Pointer(&persistFile)),
	)
	if err := hresultError("QueryInterface(IPersistFile)", result); err != nil {
		return err
	}
	defer comRelease(persistFile)

	linkPtr, err := syscall.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	result, _, _ = syscall.SyscallN(comMethod(persistFile, 6), uintptr(persistFile), uintptr(unsafe.Pointer(linkPtr)), 1)
	if err := hresultError("IPersistFile.Save", result); err != nil {
		return err
	}

	// Informa ao Explorer que o atalho foi regravado. Isso força a Área de
	// Trabalho e o Menu Iniciar a descartarem imediatamente o ícone em cache.
	pSHChangeNotify.Call(shcneUpdateItem, shcnfPathW, uintptr(unsafe.Pointer(linkPtr)), 0)
	return nil
}
