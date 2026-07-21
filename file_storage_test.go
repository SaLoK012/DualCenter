package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := replaceFileAtomically(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFileAtomically(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("conteúdo = %q; esperado %q", data, "new")
	}
	if _, err := os.Stat(path + ".previous"); !os.IsNotExist(err) {
		t.Fatalf("backup temporário não foi removido: %v", err)
	}
}
