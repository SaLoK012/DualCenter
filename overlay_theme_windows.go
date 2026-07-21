// SPDX-License-Identifier: MIT
//go:build windows

package main

import "math"

const panelTitleFont = "Century Gothic"

// As conversões e métricas visuais ficam centralizadas para que todos os
// componentes mantenham a mesma escala, espessura e geometria.
func rgb(r, g, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

func scaled(v int32) int32 {
	return int32(math.Round(float64(v) * state.scale))
}

func scaledStroke(v int32) int32 {
	n := scaled(v)
	if n < 1 {
		return 1
	}
	return n
}

func panelCornerRadius() int32 {
	return scaled(24)
}
