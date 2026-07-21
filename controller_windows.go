// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"encoding/binary"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func processRawInput(lParam uintptr) {
	// O caminho comum usa a pilha e uma única transição para user32. Quando o
	// Windows agrupa vários relatórios HID no mesmo WM_INPUT, fazemos uma segunda
	// leitura com o tamanho informado em vez de descartar o lote inteiro.
	var stack [512]byte
	hs := uint32(unsafe.Sizeof(rawInputHeader{}))
	n := uint32(len(stack))
	r, _, _ := pGetRawInputData.Call(lParam, RID_INPUT, uintptr(unsafe.Pointer(&stack[0])), uintptr(unsafe.Pointer(&n)), uintptr(hs))
	buffer := stack[:]
	if r == ^uintptr(0) {
		size, ok := rawInputFallbackSize(n, len(stack))
		if !ok {
			return
		}
		buffer = make([]byte, size)
		n = uint32(len(buffer))
		r, _, _ = pGetRawInputData.Call(lParam, RID_INPUT, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&n)), uintptr(hs))
	}
	if r == ^uintptr(0) || r < uintptr(hs+8) || r > uintptr(len(buffer)) {
		return
	}
	b := buffer[:int(r)]
	h := (*rawInputHeader)(unsafe.Pointer(&b[0]))
	if h.Type != RIM_TYPEHID {
		return
	}
	runtime := rawInputRuntimeForDevice(h.Device)
	if runtime == nil {
		return
	}
	off := int(hs)
	sz := int(binary.LittleEndian.Uint32(b[off : off+4]))
	cnt := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
	start := off + 8
	if !rawInputBatchFits(len(b), start, sz, cnt) {
		return
	}
	for i := 0; i < cnt; i++ {
		processDualSenseReport(h.Device, runtime, b[start+i*sz:start+(i+1)*sz])
	}
}

var (
	deviceCache          = map[uintptr]bool{}
	deviceModelCache     = map[uintptr]int{}
	bluetoothDeviceCache = map[uintptr]bool{}
	bluetoothDeviceKnown = map[uintptr]bool{}
	unknownReportLog     = map[uintptr]time.Time{}
	rawInputRuntimes     = map[uintptr]*rawInputRuntime{}
	lastRawInputRuntime  *rawInputRuntime
)

func forgetRawInputDevice(device uintptr) {
	delete(deviceCache, device)
	delete(deviceModelCache, device)
	delete(bluetoothDeviceCache, device)
	delete(bluetoothDeviceKnown, device)
	delete(unknownReportLog, device)
	delete(rawInputRuntimes, device)
	if lastRawInputRuntime != nil && lastRawInputRuntime.device == device {
		lastRawInputRuntime = nil
	}
}

func dualSenseModelFromPath(path string) int {
	p := strings.ToLower(path)
	if strings.Contains(p, "pid_0df2") || strings.Contains(p, "pid&0df2") {
		return controllerModelEdge
	}
	if strings.Contains(p, "pid_0ce6") || strings.Contains(p, "pid&0ce6") {
		return controllerModelStandard
	}
	return controllerModelUnknown
}

func dualSenseModel(device uintptr) int {
	if model, ok := deviceModelCache[device]; ok && model != controllerModelUnknown {
		return model
	}
	var info [32]byte
	binary.LittleEndian.PutUint32(info[:4], uint32(len(info)))
	n := uint32(len(info))
	r, _, _ := pGetRawInputDeviceInfoW.Call(device, RIDI_DEVICEINFO, uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(&n)))
	if r != ^uintptr(0) && n >= 24 {
		vid := binary.LittleEndian.Uint32(info[8:12])
		pid := binary.LittleEndian.Uint32(info[12:16])
		if vid == 0x054c {
			switch pid {
			case 0x0df2:
				deviceModelCache[device] = controllerModelEdge
				return controllerModelEdge
			case 0x0ce6:
				deviceModelCache[device] = controllerModelStandard
				return controllerModelStandard
			}
		}
	}
	model := dualSenseModelFromPath(getRawInputDeviceName(device))
	if model != controllerModelUnknown {
		deviceModelCache[device] = model
	}
	return model
}

func getRawInputDeviceName(device uintptr) string {
	var n uint32
	pGetRawInputDeviceInfoW.Call(device, RIDI_DEVICENAME, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	r, _, _ := pGetRawInputDeviceInfoW.Call(device, RIDI_DEVICENAME, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r == ^uintptr(0) {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func rawPathLooksDualSense(path string) bool {
	p := strings.ToLower(path)
	sony := strings.Contains(p, "vid_054c") || strings.Contains(p, "vid&0002054c") || strings.Contains(p, "vid&054c")
	product := strings.Contains(p, "pid_0ce6") || strings.Contains(p, "pid&0ce6") ||
		strings.Contains(p, "pid_0df2") || strings.Contains(p, "pid&0df2")
	return sony && product
}

func isDualSenseDevice(device uintptr) bool {
	if deviceCache[device] {
		return true
	}
	var info [32]byte
	binary.LittleEndian.PutUint32(info[:4], uint32(len(info)))
	n := uint32(len(info))
	r, _, _ := pGetRawInputDeviceInfoW.Call(device, RIDI_DEVICEINFO, uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(&n)))
	if r != ^uintptr(0) && n >= 24 {
		vid := binary.LittleEndian.Uint32(info[8:12])
		pid := binary.LittleEndian.Uint32(info[12:16])
		if vid == 0x054c && (pid == 0x0ce6 || pid == 0x0df2) {
			deviceCache[device] = true
			if pid == 0x0df2 {
				deviceModelCache[device] = controllerModelEdge
			} else {
				deviceModelCache[device] = controllerModelStandard
			}
			return true
		}
	}
	// Algumas pilhas Bluetooth entregam RIDI_DEVICEINFO incompleto no primeiro
	// pacote. O caminho HID ainda contém os identificadores do DualSense.
	if path := getRawInputDeviceName(device); rawPathLooksDualSense(path) {
		deviceCache[device] = true
		if model := dualSenseModelFromPath(path); model != controllerModelUnknown {
			deviceModelCache[device] = model
		}
		return true
	}
	// Não armazenamos resultados negativos: o Windows pode completar os dados
	// do dispositivo alguns milissegundos depois da conexão.
	return false
}

func isBluetoothDevice(device uintptr) bool {
	if bluetoothDeviceKnown[device] {
		return bluetoothDeviceCache[device]
	}
	path := strings.ToLower(getRawInputDeviceName(device))
	bt := strings.Contains(path, "bthenum") || strings.Contains(path, "bluetooth") ||
		strings.Contains(path, "vid&0002054c") ||
		strings.Contains(path, "{00001124-0000-1000-8000-00805f9b34fb}")
	bluetoothDeviceKnown[device] = true
	bluetoothDeviceCache[device] = bt
	return bt
}

func rawInputRuntimeForDevice(device uintptr) *rawInputRuntime {
	if lastRawInputRuntime != nil && lastRawInputRuntime.device == device {
		return lastRawInputRuntime
	}
	if runtime := rawInputRuntimes[device]; runtime != nil {
		lastRawInputRuntime = runtime
		return runtime
	}
	if !isDualSenseDevice(device) {
		return nil
	}
	runtime := &rawInputRuntime{device: device, bluetooth: isBluetoothDevice(device)}
	rawInputRuntimes[device] = runtime
	lastRawInputRuntime = runtime
	return runtime
}

func activateDualSenseEnhancedReports(path string) {
	p := utf16Ptr(path)
	h, _, openErr := pCreateFileW.Call(uintptr(unsafe.Pointer(p)), GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, 0, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, 0)
	if h == INVALID_HANDLE_VALUE {
		h, _, openErr = pCreateFileW.Call(uintptr(unsafe.Pointer(p)), 0, FILE_SHARE_READ|FILE_SHARE_WRITE, 0, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, 0)
	}
	if h == INVALID_HANDLE_VALUE {
		logf("Bluetooth: não foi possível abrir o HID para ativar relatórios completos: %v", openErr)
		return
	}
	defer pCloseHandle.Call(h)

	// Estes feature reports habilitam o relatório Bluetooth completo (0x31),
	// necessário para bateria e todos os botões.
	success := false
	for _, spec := range []struct {
		id byte
		n  int
	}{{0x05, 41}, {0x09, 20}, {0x20, 64}} {
		buf := make([]byte, spec.n)
		buf[0] = spec.id
		r, _, e := pHidDGetFeature.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r != 0 {
			success = true
		} else {
			logf("Bluetooth: HidD_GetFeature %#x falhou: %v", spec.id, e)
		}
	}
	if success {
		logf("Bluetooth: solicitação de relatórios completos enviada")
	}
}

func logUnknownDualSenseReport(device uintptr, report []byte) {
	now := time.Now()
	if last := unknownReportLog[device]; !last.IsZero() && now.Sub(last) < 5*time.Second {
		return
	}
	unknownReportLog[device] = now
	n := len(report)
	if n > 16 {
		n = 16
	}
	logf("Relatório DualSense não reconhecido: tamanho=%d dados=% x", len(report), report[:n])
}

func controllerLocked(device uintptr) *controllerRuntime {
	c := state.controllers[device]
	if c == nil {
		c = &controllerRuntime{dpad: 8, connection: "Indisponível", model: dualSenseModel(device)}
		state.controllers[device] = c
	}
	return c
}

func selectControllerLocked(device uintptr, c *controllerRuntime) {
	state.activeDevice = device
	state.batteryKnown = c.batteryKnown
	state.batteryPercent = c.batteryPercent
	state.charging = c.charging
	state.connection = c.connection
	state.controllerModel = c.model
}

func selectAnyControllerLocked() {
	state.activeDevice = 0
	for device, c := range state.controllers {
		selectControllerLocked(device, c)
		return
	}
	state.batteryKnown = false
	state.batteryPercent = 0
	state.charging = false
	state.connection = "Indisponível"
	state.controllerModel = controllerModelUnknown
}

func menuControllerInputEnabledLocked(device uintptr) bool {
	return state.overlayMode == overlayMenu && state.menuDevice == device && !state.gameDialogOpen
}

func parseBatteryStatus(status byte) (known bool, percent int, charging bool) {
	raw := int(status & 0x0f)
	cs := int(status >> 4 & 0x0f)
	switch cs {
	case 0:
		return true, clamp(raw*10+5, 0, 100), false
	case 1:
		return true, clamp(raw*10+5, 0, 100), true
	case 2:
		return true, 100, true
	default:
		return false, 0, false
	}
}

func processDualSenseReport(device uintptr, runtime *rawInputRuntime, report []byte) {
	if len(report) < 7 {
		return
	}
	bluetooth := runtime.bluetooth
	var face byte
	var ps bool
	var batteryStatus byte
	var recognized, full, hasBattery, wantsEnhanced bool
	connection := "Indisponível"

	switch {
	case report[0] == 0x31 && len(report) >= 55:
		// Bluetooth completo: Report ID 0x31 + contador no byte seguinte.
		off := 2
		face = report[off+7]
		ps = report[off+9]&1 != 0
		batteryStatus = report[off+52]
		recognized, full, hasBattery = true, true, true
		connection = "Bluetooth"

	case report[0] == 0x01 && len(report) >= 10 && (bluetooth || len(report) == 10 || len(report) == 78):
		// Bluetooth simples. Alguns drivers preenchem o buffer até o tamanho
		// máximo da coleção, então não exigimos tamanho exatamente igual a 10.
		face = report[5]
		ps = report[7]&1 != 0
		recognized, wantsEnhanced = true, true
		connection = "Bluetooth"

	case !bluetooth && report[0] == 0x01 && len(report) >= 54:
		// USB completo.
		off := 1
		face = report[off+7]
		ps = report[off+9]&1 != 0
		batteryStatus = report[off+52]
		recognized, full, hasBattery = true, true, true
		connection = "USB"

	case bluetooth && len(report) >= 77:
		// Variante Bluetooth sem Report ID no bloco Raw Input.
		off := 1
		face = report[off+7]
		ps = report[off+9]&1 != 0
		batteryStatus = report[off+52]
		recognized, full, hasBattery = true, true, true
		connection = "Bluetooth"

	case !bluetooth && len(report) >= 63:
		// Variante USB sem Report ID no bloco Raw Input.
		off := 0
		face = report[off+7]
		ps = report[off+9]&1 != 0
		batteryStatus = report[off+52]
		recognized, full, hasBattery = true, true, true
		connection = "USB"

	case len(report) >= 9 && len(report) <= 16:
		// Bluetooth simples sem Report ID.
		face = report[4]
		ps = report[6]&1 != 0
		recognized, wantsEnhanced = true, true
		connection = "Bluetooth"
	}

	if !recognized {
		logUnknownDualSenseReport(device, report)
		return
	}

	// Ignora relatórios idênticos. Analógicos, giroscópio e touchpad variam o
	// tempo todo, mas não são usados pelo DualCenter; processar apenas PS, direcional,
	// X, O, conexão e bateria reduz bastante o consumo em segundo plano.
	fast := &runtime.report
	navFace := face & 0xef
	inputChangedFast := !fast.initialized || fast.face != navFace || fast.ps != ps
	connectionChangedFast := !fast.initialized || fast.connection != connection
	batteryChangedFast := hasBattery && (!fast.hasBattery || fast.batteryStatus != batteryStatus)
	maintenance := false
	if wantsEnhanced {
		if fast.enhancedCountdown == 0 {
			// A primeira solicitação continua imediata. As verificações seguintes
			// são apenas manutenção e não precisam bloquear o estado a cada 64
			// relatórios enquanto aguardam o prazo de nova tentativa.
			fast.enhancedCountdown = 255
			maintenance = true
		} else {
			fast.enhancedCountdown--
		}
	}
	fast.initialized = true
	fast.face = navFace
	fast.ps = ps
	fast.connection = connection
	if hasBattery {
		fast.hasBattery = true
		fast.batteryStatus = batteryStatus
	}
	if !inputChangedFast && !connectionChangedFast && !batteryChangedFast && !maintenance {
		// Enquanto o PS está segurado, apenas renova a marca de atividade para o
		// timer de pressão longa. Uma renovação a cada 32 relatórios mantém ampla
		// margem para detectar desconexão sem bloquear o mutex em todo pacote HID.
		if ps {
			if fast.psRefreshCountdown > 0 {
				fast.psRefreshCountdown--
				return
			}
			fast.psRefreshCountdown = 31
			state.mu.Lock()
			if c := state.controllers[device]; c != nil && c.psDown {
				c.lastReportAt = time.Now()
			}
			state.mu.Unlock()
		}
		return
	}
	if !ps {
		fast.psRefreshCountdown = 0
	}

	startEnhancedRequest := false
	needsRedraw := false
	model := dualSenseModel(device)
	now := time.Now()
	state.mu.Lock()
	c := controllerLocked(device)
	modelChanged := model != controllerModelUnknown && c.model != model
	if modelChanged {
		c.model = model
	}
	connectionChanged := c.connection != connection
	if c.connection != connection {
		c.connection = connection
	}
	batteryChanged := false
	if hasBattery {
		known, percent, charging := parseBatteryStatus(batteryStatus)
		if c.batteryKnown != known || c.batteryPercent != percent || c.charging != charging {
			c.batteryKnown = known
			c.batteryPercent = percent
			c.charging = charging
			batteryChanged = true
		}
	}
	if state.activeDevice == 0 || (state.activeDevice == device && (connectionChanged || batteryChanged || modelChanged)) {
		selectControllerLocked(device, c)
		needsRedraw = true
	}
	if full && connection == "Bluetooth" {
		delete(state.btRequest, device)
		delete(state.btAttempts, device)
	}
	if wantsEnhanced && maintenance {
		lastTry := state.btRequest[device]
		wait := 2 * time.Second
		if state.btAttempts[device] >= 3 {
			wait = 30 * time.Second
		}
		if lastTry.IsZero() || now.Sub(lastTry) >= wait {
			state.btRequest[device] = now
			state.btAttempts[device]++
			startEnhancedRequest = true
		}
	}

	dpad := face & 0x0f
	cross := face&0x20 != 0
	circle := face&0x40 != 0
	triangle := face&0x80 != 0
	prevDpad, prevCross, prevCircle, prevTriangle := c.dpad, c.cross, c.circle, c.triangle

	psChanged := false
	if ps && !c.psDown {
		c.psDown = true
		c.psDownAt = now
		c.lastReportAt = now
		c.longTriggered = false
		psChanged = true
		handlePSPressLocked(device, c, now)
	} else if ps && c.psDown {
		c.lastReportAt = now
	} else if !ps && c.psDown {
		c.psDown = false
		c.lastPSRelease = now
		psChanged = true
	}

	menuActionHandled := false
	if menuActionPressed(dpad, prevDpad, cross, prevCross, circle, prevCircle, triangle, prevTriangle) && menuControllerInputEnabledLocked(device) {
		handleMenuButtonsLocked(dpad, prevDpad, cross, prevCross, circle, prevCircle, triangle, prevTriangle)
		menuActionHandled = true
	}
	c.dpad = dpad
	c.cross = cross
	c.circle = circle
	c.triangle = triangle
	if shouldCompleteCircleRelease(state.circleReleasePending, state.circleReleaseDevice, device, circle) {
		completeCircleReleaseLocked(true)
	}
	if batteryChanged && c.batteryKnown {
		if c.charging || c.batteryPercent > 25 {
			state.lastLowBatteryAlert = time.Time{}
		} else if state.batteryAlerts && batteryOverlayEnabled(state.hideBatteryOverlay) && state.overlayMode == overlayHidden && !state.circleReleasePending {
			if state.lastLowBatteryAlert.IsZero() || now.Sub(state.lastLowBatteryAlert) >= 15*time.Minute {
				state.lastLowBatteryAlert = now
				selectControllerLocked(device, c)
				showBatteryLocked()
			}
		}
	}
	if psChanged {
		updateTimerIntervalLocked()
	}
	state.mu.Unlock()
	flushPendingOverlayShow()

	if menuActionHandled {
		// Invalide a janela e deixe o wndProc compor o frame depois que o relatório
		// Raw Input terminar. A renderização síncrona aqui bloqueava a entrada do
		// controle e ainda duplicava o WM_PAINT já solicitado pelos manipuladores.
		redraw()
	} else if needsRedraw {
		redraw()
	}
	if startEnhancedRequest {
		path := getRawInputDeviceName(device)
		if path != "" {
			go activateDualSenseEnhancedReports(path)
		}
	}
}

func handlePSPressLocked(device uintptr, c *controllerRuntime, now time.Time) {
	selectControllerLocked(device, c)

	// Alguns drivers do DualSense alternam relatórios simples/completos quando o PS
	// é pressionado. Sem esta trava, um único toque pode virar dois eventos: o menu
	// pisca no topo por um frame e só depois aparece o overlay de bateria.
	if !state.lastPSActionAt.IsZero() && now.Sub(state.lastPSActionAt) < psDuplicateGuard {
		return
	}
	// Exige uma soltura minimamente estável para contar o segundo toque como duplo.
	// Oscilações rápidas do relatório HID durante o mesmo toque são descartadas.
	if !c.lastPSPress.IsZero() && !c.lastPSRelease.IsZero() && now.Sub(c.lastPSRelease) < psReleaseStableGap {
		return
	}
	state.lastPSActionAt = now

	if state.overlayMode == overlayMenu {
		if state.menuDevice != device {
			return
		}
		// O seletor de executável é modal e pertence ao DualCenter. Enquanto ele
		// estiver aberto, não deixe o PS fechar a janela proprietária nem devolver
		// o foco ao jogo por baixo.
		if state.gameDialogOpen {
			c.longTriggered = true
			c.lastPSPress = time.Time{}
			return
		}
		c.lastPSPress = time.Time{}
		closeMenuLocked()
		return
	}

	if !c.lastPSPress.IsZero() && now.Sub(c.lastPSPress) <= psDoublePressWindow {
		c.lastPSPress = time.Time{}
		if exclusiveFullscreenActive() {
			logf("overlay de abas bloqueado: tela cheia exclusiva detectada")
			showMessageLocked(fullscreenUnavailableTitle, fullscreenUnavailableBody, 3*time.Second)
		} else {
			showMenuLocked(device)
		}
		return
	}

	c.lastPSPress = now
	if batteryOverlayEnabled(state.hideBatteryOverlay) {
		showBatteryLocked()
	} else {
		updateTimerIntervalLocked()
	}
}
