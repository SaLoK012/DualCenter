//go:build windows

package main

import (
	"syscall"
)

var (
	powrprof         = syscall.NewLazyDLL("powrprof.dll")
	pSetSuspendState = powrprof.NewProc("SetSuspendState")
)

// executeSuspendAction usa a API nativa de energia. O primeiro argumento
// falso solicita suspensão, nunca hibernação; os demais preservam eventos de
// despertar e evitam o modo crítico forçado.
func executeSuspendAction() {
	ok, _, callErr := pSetSuspendState.Call(0, 0, 0)
	if ok == 0 {
		if errno, valid := callErr.(syscall.Errno); valid && errno != 0 {
			logf("falha ao suspender o computador: SetSuspendState: %v", errno)
		} else {
			logf("falha ao suspender o computador: SetSuspendState retornou falso")
		}
	}
}
