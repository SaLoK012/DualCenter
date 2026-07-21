package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogIfNeeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DualCenter.log")
	content := []byte("registro antigo")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	rotateLogIfNeeded(path, int64(len(content)))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("current log still exists after rotation: %v", err)
	}
	backup, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(content) {
		t.Fatalf("backup content = %q, want %q", backup, content)
	}
}
