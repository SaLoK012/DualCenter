package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestParseICO(t *testing.T) {
	data := make([]byte, 26)
	binary.LittleEndian.PutUint16(data[2:4], 1)
	binary.LittleEndian.PutUint16(data[4:6], 1)
	data[6], data[7], data[10], data[11] = 16, 16, 1, 0
	binary.LittleEndian.PutUint16(data[12:14], 32)
	binary.LittleEndian.PutUint32(data[14:18], 4)
	binary.LittleEndian.PutUint32(data[18:22], 22)
	copy(data[22:], []byte{1, 2, 3, 4})
	path := filepath.Join(t.TempDir(), "icon.ico")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	images, err := parseICO(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0].id != 1 || !bytes.Equal(images[0].data, []byte{1, 2, 3, 4}) {
		t.Fatalf("ICO interpretado incorretamente: %#v", images)
	}
}

func TestVersionInfoUsesCentralMetadata(t *testing.T) {
	blob, err := buildVersionInfo("DualCenter", "DualCenter.exe")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"1.0.0.30 Oficial", "DualCenter.exe", "SaLoK"} {
		if !bytes.Contains(blob, utf16z(expected)) {
			t.Fatalf("VERSIONINFO não contém %q", expected)
		}
	}
}

func TestResourceSectionCreatesRelocationForEveryItem(t *testing.T) {
	types := []*resourceType{{id: 24, items: []*resourceItem{{id: 1, data: []byte("manifest")}}}}
	section, relocations := buildResourceSection(types)
	if len(section) == 0 || len(relocations) != 1 {
		t.Fatalf("seção=%d bytes; relocações=%d", len(section), len(relocations))
	}
}
