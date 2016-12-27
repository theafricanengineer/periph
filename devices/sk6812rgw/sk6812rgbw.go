// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package sk6812rgbw

import (
	"errors"
	"image"
	"image/color"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/stream"
	"periph.io/x/periph/devices"
)

// Dev is a handle to the LED strip.
type Dev struct {
	p         gpio.PinStreamer
	numLights int
	b         stream.Bits
}

// ColorModel implements devices.Display. There's no surprise, it is
// color.NRGBAModel.
func (d *Dev) ColorModel() color.Model {
	return color.NRGBAModel
}

// Bounds implements devices.Display. Min is guaranteed to be {0, 0}.
func (d *Dev) Bounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{X: d.numLights, Y: 1}}
}

// Draw implements devices.Display.
//
// Using something else than image.NRGBA is 10x slower and is not recommended.
// The alpha channel is ignored and the internal driver allocates the power
// automatically to the white channel as needed.
func (d *Dev) Draw(r image.Rectangle, src image.Image, sp image.Point) {
	r = r.Intersect(d.Bounds())
	srcR := src.Bounds()
	srcR.Min = srcR.Min.Add(sp)
	if dX := r.Dx(); dX < srcR.Dx() {
		srcR.Max.X = srcR.Min.X + dX
	}
	if dY := r.Dy(); dY < srcR.Dy() {
		srcR.Max.Y = srcR.Min.Y + dY
	}
	//rasterImg(d.buf, r, src, srcR)
	//_, _ = d.s.Write(d.buf)
}

// Write accepts a stream of raw RGBW pixels and sends it as NRZ encoded
// stream.
func (d *Dev) Write(pixels []byte) (int, error) {
	if len(pixels)%3 != 0 {
		return 0, errLength
	}
	raster(d.b.Bits, pixels)
	err := d.p.Stream(&d.b)
	return len(pixels), err
}

// New opens a handle to a SK6812RGBW or SK6812RGBWW.
//
// `speed` can be up to 800000.
func New(p gpio.PinIO, numLights, speed int) (*Dev, error) {
	s, ok := p.(gpio.PinStreamer)
	if !ok {
		return nil, errors.New("sk6812rgbw: pin must implement gpio.PinStreamer")
	}
	if speed == 0 {
		speed = 400000
	}
	return &Dev{
		p:         s,
		numLights: numLights,
		b: stream.Bits{
			Res:  time.Second / time.Duration(speed),
			Bits: make(gpio.Bits, numLights*4*4),
		},
	}, nil
}

//

var errLength = errors.New("sk6218rgbw: invalid RGB stream length")

// expandNRZ converts a 8 bit channel intensity into the encoded 24 bits.
func expandNRZ(b byte) uint32 {
	// The stream is 1x01x01x01x01x01x01x01x0 with the x bits being the bits from
	// `b` in reverse order.
	out := uint32(0x924924)
	out |= uint32(b&0x80) << (3*7 + 1 - 7)
	out |= uint32(b&0x40) << (3*6 + 1 - 6)
	out |= uint32(b&0x20) << (3*5 + 1 - 5)
	out |= uint32(b&0x10) << (3*4 + 1 - 4)
	out |= uint32(b&0x08) << (3*3 + 1 - 3)
	out |= uint32(b&0x04) << (3*2 + 1 - 2)
	out |= uint32(b&0x02) << (3*1 + 1 - 1)
	out |= uint32(b&0x01) << (3*0 + 1 - 0)
	return out
}

// raster converts a RGB input stream into a binary output stream as it must be
// sent over the GPIO pin.
//
// `in` is RGB 24 bits. Each bit is encoded over 3 bits so the length of `out`
// must be 3x as large as `in`.
//
// The encoding is NRZ: https://en.wikipedia.org/wiki/Non-return-to-zero
func raster(out, in []byte) {
	for i := 0; i < len(in); i += 3 {
		// Encoded format is GRB as 72 bits.
		g := expandNRZ(in[i+1])
		out[3*i+0] = byte(g >> 16)
		out[3*i+0] = byte(g >> 8)
		out[3*i+0] = byte(g)
		r := expandNRZ(in[i])
		out[3*i+0] = byte(r >> 16)
		out[3*i+0] = byte(r >> 8)
		out[3*i+0] = byte(r)
		b := expandNRZ(in[i+2])
		out[3*i+0] = byte(b >> 16)
		out[3*i+0] = byte(b >> 8)
		out[3*i+0] = byte(b)
	}
}

var _ devices.Display = &Dev{}
