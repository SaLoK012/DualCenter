// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"strings"
	"unsafe"
)

func initAudioEndpoint() {
	r, _, _ := pCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	if succeeded(r) {
		audioCOMInitialized = true
	}
	bindDefaultAudioEndpoint()
}

func shutdownAudio() {
	if audioEndpoint != nil {
		comRelease(audioEndpoint)
		audioEndpoint = nil
	}
	if audioCOMInitialized {
		pCoUninitialize.Call()
		audioCOMInitialized = false
	}
}

func comRelease(o unsafe.Pointer) {
	if o != nil {
		vcall(o, 2)
	}
}

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioEndpointVolume = guid{0x5CDF2C82, 0x841E, 0x4546, [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
	iidIPolicyConfig        = guid{0xF8679F50, 0x850A, 0x41CF, [8]byte{0x9C, 0x72, 0x43, 0x0F, 0x29, 0x02, 0x90, 0xC8}}
	clsidPolicyConfig       = guid{0x870AF99C, 0x171D, 0x4F9E, [8]byte{0xAF, 0x0D, 0xE6, 0x3D, 0xF4, 0x0C, 0x2B, 0xC9}}
	iidIPolicyConfigVista   = guid{0x568B9108, 0x44BF, 0x40B4, [8]byte{0x90, 0x06, 0x86, 0xAF, 0xE5, 0xB5, 0xA6, 0x20}}
	clsidPolicyConfigVista  = guid{0x294935CE, 0xF637, 0x4E7C, [8]byte{0xA4, 0x1B, 0xAB, 0x25, 0x54, 0x60, 0xB8, 0x62}}
	pkeyFriendlyName        = propertyKey{guid{0xA45C254E, 0xDF1C, 0x4EFD, [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}
)

func createEnumerator() unsafe.Pointer {
	var o unsafe.Pointer
	r, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, CLSCTX_ALL, uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&o)))
	if !succeeded(r) {
		return nil
	}
	return o
}

func bindDefaultAudioEndpoint() {
	if audioEndpoint != nil {
		comRelease(audioEndpoint)
		audioEndpoint = nil
	}
	en := createEnumerator()
	if en == nil {
		return
	}
	defer comRelease(en)
	var dev unsafe.Pointer
	if !succeeded(vcall(en, 4, E_RENDER, 1, uintptr(unsafe.Pointer(&dev)))) || dev == nil {
		return
	}
	defer comRelease(dev)
	var ep unsafe.Pointer
	if succeeded(vcall(dev, 3, uintptr(unsafe.Pointer(&iidIAudioEndpointVolume)), CLSCTX_ALL, 0, uintptr(unsafe.Pointer(&ep)))) {
		audioEndpoint = ep
	}
}

func getVolume() (float32, bool) {
	if audioEndpoint == nil {
		bindDefaultAudioEndpoint()
	}
	if audioEndpoint == nil {
		return 0, false
	}
	var v float32
	var m int32
	if !succeeded(vcall(audioEndpoint, 9, uintptr(unsafe.Pointer(&v)))) {
		return 0, false
	}
	vcall(audioEndpoint, 15, uintptr(unsafe.Pointer(&m)))
	return v, m != 0
}

func sendMediaKey(vk uintptr) {
	pKeybdEvent.Call(vk, 0, 0, 0)
	pKeybdEvent.Call(vk, 0, 0x0002, 0)
}

func volumeStep(delta float32) {
	if delta > 0 {
		sendMediaKey(0xAF)
	} else {
		sendMediaKey(0xAE)
	}
	redraw()
}

func toggleMute() {
	sendMediaKey(0xAD)
	redraw()
}

func getAudioDeviceID(dev unsafe.Pointer) string {
	var p unsafe.Pointer
	if !succeeded(vcall(dev, 5, uintptr(unsafe.Pointer(&p)))) || p == nil {
		return ""
	}
	defer pCoTaskMemFree.Call(uintptr(p))
	return utf16StringFromPointer(p)
}

func getAudioDeviceFriendlyName(dev unsafe.Pointer) string {
	var store unsafe.Pointer
	if !succeeded(vcall(dev, 4, STGM_READ, uintptr(unsafe.Pointer(&store)))) || store == nil {
		return "Saída de áudio"
	}
	defer comRelease(store)
	var pv propVariant
	if !succeeded(vcall(store, 5, uintptr(unsafe.Pointer(&pkeyFriendlyName)), uintptr(unsafe.Pointer(&pv)))) {
		return "Saída de áudio"
	}
	defer pPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	if pv.Vt == 31 {
		return utf16StringFromPointer(pv.Ptr)
	}
	return "Saída de áudio"
}

func enumerateAudioOutputs() []audioDevice {
	en := createEnumerator()
	if en == nil {
		return nil
	}
	defer comRelease(en)
	var col unsafe.Pointer
	if !succeeded(vcall(en, 3, E_RENDER, DEVICE_STATE_ACTIVE, uintptr(unsafe.Pointer(&col)))) || col == nil {
		return nil
	}
	defer comRelease(col)
	var n uint32
	if !succeeded(vcall(col, 3, uintptr(unsafe.Pointer(&n)))) {
		return nil
	}
	out := make([]audioDevice, 0, n)
	for i := uint32(0); i < n; i++ {
		var d unsafe.Pointer
		if !succeeded(vcall(col, 4, uintptr(i), uintptr(unsafe.Pointer(&d)))) || d == nil {
			continue
		}
		id := getAudioDeviceID(d)
		name := getAudioDeviceFriendlyName(d)
		comRelease(d)
		if id != "" {
			out = append(out, audioDevice{id, name})
		}
	}
	return out
}

func defaultAudioID() string {
	en := createEnumerator()
	if en == nil {
		return ""
	}
	defer comRelease(en)
	var d unsafe.Pointer
	if !succeeded(vcall(en, 4, E_RENDER, 1, uintptr(unsafe.Pointer(&d)))) || d == nil {
		return ""
	}
	defer comRelease(d)
	return getAudioDeviceID(d)
}

func currentAudioOutputIndex(a []audioDevice) int {
	id := defaultAudioID()
	for i := range a {
		if strings.EqualFold(a[i].ID, id) {
			return i
		}
	}
	return 0
}

func setDefaultAudioEndpointWith(clsid, iid *guid, method int, id string) bool {
	var pc unsafe.Pointer
	r, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, CLSCTX_ALL, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&pc)))
	if !succeeded(r) || pc == nil {
		return false
	}
	defer comRelease(pc)
	p := utf16Ptr(id)
	ok := true
	for role := 0; role < 3; role++ {
		if !succeeded(vcall(pc, method, uintptr(unsafe.Pointer(p)), uintptr(role))) {
			ok = false
		}
	}
	return ok
}

func setDefaultAudioEndpoint(id string) bool {
	// Implementação atual e fallback Vista para diferentes versões do Windows.
	if setDefaultAudioEndpointWith(&clsidPolicyConfig, &iidIPolicyConfig, 13, id) {
		return strings.EqualFold(defaultAudioID(), id)
	}
	if setDefaultAudioEndpointWith(&clsidPolicyConfigVista, &iidIPolicyConfigVista, 12, id) {
		return strings.EqualFold(defaultAudioID(), id)
	}
	return false
}
