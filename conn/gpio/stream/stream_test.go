// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stream

import (
	"testing"
	"time"

	"periph.io/x/periph/conn/gpio"
)

func TestBits(t *testing.T) {
	b := Bits{Res: time.Second, Bits: make(gpio.Bits, 100)}
	if b.Resolution() != b.Res {
		t.FailNow()
	}
	if b.Duration() != 100*time.Second {
		t.FailNow()
	}
}

/*
func TestBits_Raster8(t *testing.T) {
	b := Bits{Res: time.Second, Bits: make(gpio.Bits, 100)}
	// TODO(maruel): Test all code path, including filtering and all errors.
	var d8 []uint8
	mask := uint8(0)
	if err := b.raster8(8*time.Millisecond, d8, mask); err == nil {
		t.FailNow()
	}
}
*/

func TestBits_Raster32(t *testing.T) {
	b := Bits{Res: time.Second, Bits: make(gpio.Bits, 100)}
	// TODO(maruel): Test all code path, including filtering and all errors.
	setMask := uint32(0)
	clearMask := uint32(0)
	var d32Set []uint32
	var d32Clear []uint32
	if err := b.raster32(8*time.Millisecond, d32Set, d32Clear, setMask, clearMask); err == nil {
		t.FailNow()
	}
}

func TestEdges(t *testing.T) {
	e := Edges{Res: time.Second, Edges: []time.Duration{time.Second, time.Millisecond}}
	if e.Resolution() != e.Res {
		t.FailNow()
	}
	if e.Duration() != 1001*time.Millisecond {
		t.FailNow()
	}
}

/*
func TestEdges_Raster8(t *testing.T) {
	e := Edges{Res: time.Second, Edges: []time.Duration{time.Second, time.Millisecond}}
	// TODO(maruel): Test all code path, including filtering and all errors.
	var d8 []uint8
	mask := uint8(0)
	if err := e.raster8(8*time.Millisecond, d8, mask); err == nil {
		t.FailNow()
	}
}
*/

func TestEdges_Raster32(t *testing.T) {
	e := Edges{Res: time.Second, Edges: []time.Duration{time.Second, time.Millisecond}}
	// TODO(maruel): Test all code path, including filtering and all errors.
	setMask := uint32(0)
	clearMask := uint32(0)
	var d32Set []uint32
	var d32Clear []uint32
	if err := e.raster32(8*time.Millisecond, d32Set, d32Clear, setMask, clearMask); err == nil {
		t.FailNow()
	}
}
