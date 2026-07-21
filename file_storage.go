package main

import (
	"os"
	"path/filepath"
)

// replaceFileAtomically grava no mesmo volume, preserva a versão anterior até
// a troca terminar e a restaura se o Windows recusar a substituição.
func replaceFileAtomically(path string, data []byte, perm os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	backup := path + ".previous"
	_ = os.Remove(tmp)
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(tmp)
	}()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}

	hadPrevious := false
	if _, statErr := os.Stat(path); statErr == nil {
		_ = os.Remove(backup)
		if err = os.Rename(path, backup); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if err = os.Rename(tmp, path); err != nil {
		if hadPrevious {
			_ = os.Rename(backup, path)
		}
		return err
	}
	if hadPrevious {
		_ = os.Remove(backup)
	}
	return nil
}
