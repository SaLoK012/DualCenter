// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func main() {
	uninstallMode := false
	quiet := false
	for _, arg := range os.Args[1:] {
		switch strings.ToLower(arg) {
		case "--uninstall":
			uninstallMode = true
		case "--quiet":
			quiet = true
		}
	}
	if self, err := os.Executable(); err == nil && strings.EqualFold(filepath.Base(self), "Uninstall.exe") {
		uninstallMode = true
	}
	if uninstallMode {
		if quiet {
			uninstallQuiet()
			return
		}
		os.Exit(runUninstallerGUI())
	}
	os.Exit(runInstallerGUI())
}
