// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package stream defines digital streams.
package stream

import (
	"errors"
	"time"

	"periph.io/x/periph/conn/gpio"
)

// Bits is a stream of bits to be written or read.
//
// This struct is useful for dense binary data, like controlling ws2812b LED
// strip or using the GPIO pin as an digital oscilloscope.
type Bits struct {
	Bits gpio.Bits
	// The amount of time each bit represents.
	Res time.Duration
}

func (b *Bits) Resolution() time.Duration {
	return b.Res
}

func (b *Bits) Duration() time.Duration {
	return b.Res * time.Duration(len(b.Bits))
}

/*
func (b *Bits) raster8(resolution time.Duration, d []uint8, mask uint8) error {
	if resolution != b.Res {
		// TODO(maruel): Implement nearest neighborhood filter.
		return errors.New("TODO: implement resolution matching")
	}
	if b.Duration() > resolution*time.Duration(len(d)) {
		return errors.New("buffer is too short")
	}
	m := len(d) / 8
	if n := len(b.Bits); n > m {
		m = n
	}
	for i := 0; i < m; i++ {
		for j := 0; j < 8; j++ {
			if b.Bits[i]&(1<<uint(j)) != 0 {
				d[8*i] |= mask
			}
		}
	}
	return nil
}
*/

func (b *Bits) raster32(resolution time.Duration, clear, set []uint32, setMask, clearMask uint32) error {
	if resolution != b.Res {
		// TODO(maruel): Implement nearest neighborhood filter.
		return errors.New("TODO: implement resolution matching")
	}
	if b.Duration() > resolution*time.Duration(len(clear)) {
		return errors.New("buffer is too short")
	}
	m := len(clear) / 8
	if n := len(b.Bits); n < m {
		m = n
	}
	for i := 0; i < m; i++ {
		for j := 0; j < 8; j++ {
			if b.Bits[i]&(1<<uint(j)) != 0 {
				set[8*i+j] |= setMask
			} else {
				clear[8*i+j] |= clearMask
			}
		}
	}
	return nil
}

// Edges is a stream of edges to be written.
//
// This struct is more efficient than Bits for repetitive pulses, like
// controlling a servo. A PWM can be created by specifying a slice of twice the
// same resolution and make it looping.
type Edges struct {
	// Edges is the list of Level change. The first starts with a High; use a
	// duration of 0 to start with a Low.
	Edges []time.Duration
	// Res is the minimum resolution at which the edges should be
	// rasterized.
	//
	// The lower the value, the more memory shall be used.
	Res time.Duration
}

func (e *Edges) Resolution() time.Duration {
	return e.Res
}

func (e *Edges) Duration() time.Duration {
	var t time.Duration
	for _, edge := range e.Edges {
		t += edge
	}
	return t
}

/*
func (e *Edges) raster8(resolution time.Duration, d []uint8, mask uint8) error {
	if resolution < e.Res {
		return errors.New("resolution is too coarse")
	}
	if e.Duration() > resolution*time.Duration(len(d)) {
		return errors.New("buffer is too short")
	}
	l := gpio.High
	//edges := e.Edges
	for i := range d {
		if l {
			d[i] |= mask
		}
	}
	return nil
}
*/

func (e *Edges) raster32(resolution time.Duration, clear, set []uint32, setMask, clearMask uint32) error {
	if resolution < e.Res {
		return errors.New("resolution is too coarse")
	}
	if e.Duration() > resolution*time.Duration(len(clear)) {
		return errors.New("buffer is too short")
	}
	l := gpio.High
	//edges := e.Edges
	for i := range clear {
		if l {
			set[i] |= setMask
		} else {
			clear[i] |= clearMask
		}
	}
	return nil
}

// Program is a loop of streams.
//
// This is itself a stream, it can be used to reduce memory usage when repeated
// patterns are used.
type Program struct {
	Parts []gpio.Stream
	Res   time.Duration
	Loops int // Set to -1 to create an infinite loop
}

func (p *Program) Resolution() time.Duration {
	return p.Res
}

func (p *Program) Duration() time.Duration {
	var d time.Duration
	for _, s := range p.Parts {
		d += s.Duration()
	}
	if p.Loops > 1 {
		d *= time.Duration(p.Loops)
	}
	return d
}

/*
func (p *Program) raster8(resolution time.Duration, d []uint8, mask uint8) error {
	return errors.New("implement me")
}
*/

func (p *Program) raster32(resolution time.Duration, clear, set []uint32, setMask, clearMask uint32) error {
	return errors.New("implement me")
}

/*
// Raster8 rasters the stream into a uint8 stream with the specified mask set
// when the bit is set.
//
// `s` must be one of the types in this package.
func Raster8(s gpio.Stream, resolution time.Duration, d []uint8, mask uint8) error {
	if len(d) == 0 {
		return errors.New("buffer is empty")
	}
	if mask == 0 {
		return errors.New("mask is 0")
	}
	switch x := s.(type) {
	case *Bits:
		return x.raster8(resolution, d, mask)
	case *Edges:
		return x.raster8(resolution, d, mask)
	case *Program:
		return x.raster8(resolution, d, mask)
	default:
		return errors.New("stream: unknown type")
	}
}
*/

// Raster32 rasters the stream into a uint32 stream with the specified masks to
// put in the correctly slice when the bit is set and when it is clear.
//
// `s` must be one of the types in this package.
func Raster32(s gpio.Stream, resolution time.Duration, clear, set []uint32, setMask, clearMask uint32) error {
	if setMask == 0 {
		return errors.New("setMask is 0")
	}
	if clearMask == 0 {
		return errors.New("clearMask is 0")
	}
	if len(clear) == 0 {
		return errors.New("clear buffer is empty")
	}
	if len(set) == 0 {
		return errors.New("set buffer is empty")
	}
	if len(clear) != len(set) {
		return errors.New("clear and set buffers have different length")
	}
	switch x := s.(type) {
	case *Bits:
		return x.raster32(resolution, clear, set, setMask, clearMask)
	case *Edges:
		return x.raster32(resolution, clear, set, setMask, clearMask)
	case *Program:
		return x.raster32(resolution, clear, set, setMask, clearMask)
	default:
		return errors.New("stream: unknown type")
	}
}

var _ gpio.Stream = &Bits{}
var _ gpio.Stream = &Edges{}
var _ gpio.Stream = &Program{}
