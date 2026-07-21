package version

import "testing"

func TestEmbeddedVersionIsComplete(t *testing.T) {
	if Current.Version != "1.0" || Current.Build != 30 {
		t.Fatalf("versão incorporada inesperada: %#v", Current)
	}
	if got, want := FileVersion(), "1.0.0.30"; got != want {
		t.Fatalf("FileVersion() = %q; esperado %q", got, want)
	}
	if got, want := Display(), "v1.0 Oficial (Build 30)"; got != want {
		t.Fatalf("Display() = %q; esperado %q", got, want)
	}
}

func TestTwoAndThreePartPublicVersionsProduceFourPartFileVersions(t *testing.T) {
	original := Current
	defer func() { Current = original }()

	Current.Version = "1.0"
	Current.Build = 30
	if got, want := FileVersion(), "1.0.0.30"; got != want {
		t.Fatalf("versão pública curta: FileVersion() = %q; esperado %q", got, want)
	}
	Current.Version = "1.2.3"
	if got, want := FileVersion(), "1.2.3.30"; got != want {
		t.Fatalf("versão semântica completa: FileVersion() = %q; esperado %q", got, want)
	}
}

func TestMustParseRejectsIncompleteMetadata(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("metadados incompletos deveriam causar panic durante o build")
		}
	}()
	_ = mustParse([]byte(`{"version":"1.0","build":0,"publisher":""}`))
}
