// SPDX-License-Identifier: MIT

package main

const maxRawInputBufferSize = 64 << 10

// shouldCompleteCircleRelease mantém a regra de liberação independente da API
// do Windows, para que a condição crítica possa ser testada em qualquer sistema.
func shouldCompleteCircleRelease(pending bool, pendingDevice, reportDevice uintptr, circlePressed bool) bool {
	return pending && pendingDevice != 0 && pendingDevice == reportDevice && !circlePressed
}

// batteryOverlayEnabled converte a configuração histórica de ocultação para a
// semântica positiva exibida ao usuário: ligado significa que o overlay aparece.
func batteryOverlayEnabled(hidden bool) bool {
	return !hidden
}

// menuActionPressed ignora a soltura dos botões e o retorno do direcional ao
// centro. Esses relatórios atualizam o estado físico do controle, mas não mudam
// nada na interface e não devem solicitar um novo frame do overlay.
func menuActionPressed(dpad, prevDpad byte, cross, prevCross, circle, prevCircle, triangle, prevTriangle bool) bool {
	dpadPressed := dpad != prevDpad && dpad <= 7
	return dpadPressed ||
		(cross && !prevCross) ||
		(circle && !prevCircle) ||
		(triangle && !prevTriangle)
}

// rawInputBatchFits valida a multiplicação antes de recortar o buffer. Assim,
// metadados HID inválidos não conseguem causar overflow nem panic no wndProc.
func rawInputBatchFits(bufferSize, start, reportSize, reportCount int) bool {
	if bufferSize < 0 || start < 0 || start > bufferSize || reportSize <= 0 || reportCount <= 0 {
		return false
	}
	return reportSize <= (bufferSize-start)/reportCount
}

// rawInputFallbackSize limita a alocação usada somente quando um lote HID não
// cabe no buffer rápido da pilha. O limite protege o processo de metadados
// inválidos sem descartar lotes legítimos enviados pelo Windows.
func rawInputFallbackSize(required uint32, stackSize int) (int, bool) {
	if required <= uint32(stackSize) || required > maxRawInputBufferSize {
		return 0, false
	}
	return int(required), true
}
