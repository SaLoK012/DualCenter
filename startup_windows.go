//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func startupCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	return `"` + filepath.Clean(exe) + `" --startup`, nil
}

func setStartupEnabled(enabled bool) error {
	keyPath := utf16Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`)
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(0x80000001, uintptr(unsafe.Pointer(keyPath)), 0, 0, 0, KEY_SET_VALUE|KEY_QUERY_VALUE, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return fmt.Errorf("RegCreateKeyExW=%d", r)
	}
	defer pRegCloseKey.Call(key)
	name := utf16Ptr(startupValueName)
	if !enabled {
		r, _, _ = pRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(name)))
		if r != 0 && r != ERROR_FILE_NOT_FOUND {
			return fmt.Errorf("RegDeleteValueW=%d", r)
		}
		return nil
	}
	command, err := startupCommand()
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(command)
	if err != nil {
		return err
	}
	r, _, _ = pRegSetValueExW.Call(key, uintptr(unsafe.Pointer(name)), 0, REG_SZ, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if r != 0 {
		return fmt.Errorf("RegSetValueExW=%d", r)
	}
	return nil
}

func readStartupValue() (string, bool) {
	keyPath := utf16Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`)
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(0x80000001, uintptr(unsafe.Pointer(keyPath)), 0, 0, 0, KEY_QUERY_VALUE, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return "", false
	}
	defer pRegCloseKey.Call(key)
	name := utf16Ptr(startupValueName)
	var typ, n uint32
	r, _, _ = pRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&n)))
	if r != 0 || typ != REG_SZ || n < 2 {
		return "", false
	}
	buf := make([]uint16, (n+1)/2)
	r, _, _ = pRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func isStartupEnabled() bool {
	actual, ok := readStartupValue()
	if !ok {
		return false
	}
	expected, err := startupCommand()
	if err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)) {
		return true
	}
	legacy := strings.TrimSuffix(expected, " --startup")
	if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(legacy)) {
		_ = setStartupEnabled(true)
		return true
	}
	return false
}
