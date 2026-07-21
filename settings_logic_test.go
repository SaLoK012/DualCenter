//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestSanitizeTabOrder(t *testing.T) {
	valid := []int{tabGames, tabBattery, tabSettings, tabModeGame, tabEnergy, tabVolume}
	if got := sanitizeTabOrder(valid); !reflect.DeepEqual(got, valid) {
		t.Fatalf("valid order changed: got %v, want %v", got, valid)
	}

	invalid := []int{tabVolume, tabVolume, tabModeGame, tabSettings, tabBattery, tabGames}
	if got, want := sanitizeTabOrder(invalid), defaultTabOrder(); !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid order was not reset: got %v, want %v", got, want)
	}
}

func TestSanitizeGames(t *testing.T) {
	games := []gameEntry{
		{Name: "  Primeiro  ", Path: ` C:\Jogos\Primeiro.exe `},
		{Name: "Duplicado", Path: `c:\jogos\primeiro.EXE`},
		{Path: `C:\Jogos\Segundo.exe`},
		{Name: "Inválido", Path: `C:\Jogos\leia-me.txt`},
	}
	got := sanitizeGames(games)
	want := []gameEntry{
		{Name: "Primeiro", Path: `C:\Jogos\Primeiro.exe`},
		{Name: "Segundo", Path: `C:\Jogos\Segundo.exe`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sanitizeGames() = %#v, want %#v", got, want)
	}
}
