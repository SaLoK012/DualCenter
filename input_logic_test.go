// SPDX-License-Identifier: MIT

package main

import "testing"

func TestCircleReleaseCompletesOnlyForTheControllerThatClosedTheMenu(t *testing.T) {
	const device uintptr = 42
	tests := []struct {
		name          string
		pending       bool
		pendingDevice uintptr
		reportDevice  uintptr
		circlePressed bool
		want          bool
	}{
		{name: "release from owner", pending: true, pendingDevice: device, reportDevice: device, want: true},
		{name: "still pressed", pending: true, pendingDevice: device, reportDevice: device, circlePressed: true},
		{name: "different controller", pending: true, pendingDevice: device, reportDevice: device + 1},
		{name: "not pending", pendingDevice: device, reportDevice: device},
		{name: "missing owner", pending: true, reportDevice: device},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldCompleteCircleRelease(test.pending, test.pendingDevice, test.reportDevice, test.circlePressed)
			if got != test.want {
				t.Fatalf("shouldCompleteCircleRelease() = %t; want %t", got, test.want)
			}
		})
	}
}

func TestBatteryOverlayEnabledUsesPositiveSemantics(t *testing.T) {
	if !batteryOverlayEnabled(false) {
		t.Fatal("overlay deveria estar ligado quando a configuração de ocultação está desligada")
	}
	if batteryOverlayEnabled(true) {
		t.Fatal("overlay deveria estar desligado quando a configuração de ocultação está ligada")
	}
}

func TestMenuActionPressedIgnoresButtonAndDpadRelease(t *testing.T) {
	for _, test := range []struct {
		name                                                         string
		dpad, prevDpad                                               byte
		cross, prevCross, circle, prevCircle, triangle, prevTriangle bool
		want                                                         bool
	}{
		{name: "dpad press", dpad: 2, prevDpad: 8, want: true},
		{name: "dpad release", dpad: 8, prevDpad: 2},
		{name: "cross press", dpad: 8, prevDpad: 8, cross: true, want: true},
		{name: "cross release", dpad: 8, prevDpad: 8, prevCross: true},
		{name: "circle press", dpad: 8, prevDpad: 8, circle: true, want: true},
		{name: "triangle press", dpad: 8, prevDpad: 8, triangle: true, want: true},
		{name: "unchanged", dpad: 8, prevDpad: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := menuActionPressed(
				test.dpad, test.prevDpad,
				test.cross, test.prevCross,
				test.circle, test.prevCircle,
				test.triangle, test.prevTriangle,
			)
			if got != test.want {
				t.Fatalf("menuActionPressed() = %t; want %t", got, test.want)
			}
		})
	}
}

func TestRawInputBatchFitsWithoutIntegerOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name                         string
		buffer, start, size, reports int
		want                         bool
	}{
		{name: "valid batch", buffer: 512, start: 32, size: 78, reports: 6, want: true},
		{name: "one byte beyond buffer", buffer: 512, start: 32, size: 481, reports: 1},
		{name: "overflowing multiplication", buffer: 512, start: 32, size: maxInt, reports: maxInt},
		{name: "invalid count", buffer: 512, start: 32, size: 78},
		{name: "start beyond buffer", buffer: 512, start: 513, size: 1, reports: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rawInputBatchFits(test.buffer, test.start, test.size, test.reports); got != test.want {
				t.Fatalf("rawInputBatchFits() = %t; want %t", got, test.want)
			}
		})
	}
}

func TestRawInputFallbackSizeIsBounded(t *testing.T) {
	for _, test := range []struct {
		name      string
		required  uint32
		stackSize int
		want      int
		ok        bool
	}{
		{name: "larger valid batch", required: 2048, stackSize: 512, want: 2048, ok: true},
		{name: "already fits stack", required: 512, stackSize: 512},
		{name: "oversized metadata", required: maxRawInputBufferSize + 1, stackSize: 512},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := rawInputFallbackSize(test.required, test.stackSize)
			if got != test.want || ok != test.ok {
				t.Fatalf("rawInputFallbackSize() = (%d, %t); want (%d, %t)", got, ok, test.want, test.ok)
			}
		})
	}
}
