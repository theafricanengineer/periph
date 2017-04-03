// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package ssd1306 controls a 128x64 monochrome OLED display via a SSD1306
// controller.
//
// The driver does differential updates, that is, it only sends modified pixels
// for the smallest rectangle, to economize bus bandwidth.
//
// The SSD1306 is a write-only device. It can be driven on either I²C or SPI.
// Changing between protocol is likely done through resistor soldering, for
// boards that support both.
//
// Datasheets
//
// Product page:
// http://www.solomon-systech.com/en/product/display-ic/oled-driver-controller/ssd1306/
//
// https://cdn-shop.adafruit.com/datasheets/SSD1306.pdf
//
// "DM-OLED096-624": https://drive.google.com/file/d/0B5lkVYnewKTGaEVENlYwbDkxSGM/view
//
// "ssd1306": https://drive.google.com/file/d/0B5lkVYnewKTGYzhyWWp0clBMR1E/view
package ssd1306

// Some have SPI enabled;
// https://hallard.me/adafruit-oled-display-driver-for-pi/
// https://learn.adafruit.com/ssd1306-oled-displays-with-raspberry-pi-and-beaglebone-black?view=all

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"

	"periph.io/x/periph/conn"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/devices"
	"periph.io/x/periph/devices/ssd1306/image1bit"
)

// FrameRate determines scrolling speed.
type FrameRate byte

// Possible frame rates.
const (
	FrameRate2   FrameRate = 7
	FrameRate3   FrameRate = 4
	FrameRate4   FrameRate = 5
	FrameRate5   FrameRate = 0
	FrameRate25  FrameRate = 6
	FrameRate64  FrameRate = 1
	FrameRate128 FrameRate = 2
	FrameRate256 FrameRate = 3
)

// Orientation is used for scrolling.
type Orientation byte

// Possible orientations for scrolling.
const (
	Left    Orientation = 0x27
	Right   Orientation = 0x26
	UpRight Orientation = 0x29
	UpLeft  Orientation = 0x2A
)

// Dev is an open handle to the display controller.
type Dev struct {
	// Communication
	c   conn.Conn
	dc  gpio.PinOut
	spi bool

	// Display size controlled by the SSD1306.
	w uint8
	h uint8

	// Mutable
	// See page 25 for the GDDRAM pages structure.
	// Narrow screen will waste the end of each page.
	// Short screen will ignore the lower pages.
	// There is 8 pages, each covering an horizontal band of 8 pixels high (1
	// byte) for 128 bytes.
	// 8*128 = 1024 bytes total for 128x64 display.
	buffer    []byte
	scrolling bool
}

// NewSPI returns a Dev object that communicates over SPI to a SSD1306 display
// controller.
//
// If rotated is true, turns the display by 180°
//
// The SSD1306 can operate at up to 3.3Mhz, which is much higher than I²C. This
// permits higher refresh rates.
//
// Wiring
//
// Connect SDA to MOSI, SCK to SCLK, CS to CS.
//
// In 3-wire SPI mode, pass nil for 'dc'. In 4-wire SPI mode, pass a GPIO pin
// to use.
//
// The RES (reset) pin can be used outside of this driver but is not supported
// natively. In case of external reset via the RES pin, this device drive must
// be reinstantiated.
func NewSPI(s spi.Conn, dc gpio.PinOut, w, h int, rotated bool) (*Dev, error) {
	if dc == gpio.INVALID {
		return nil, errors.New("ssd1306: use nil for dc to use 3-wire mode, do not use gpio.INVALID")
	}
	bits := 8
	if dc == nil {
		// 3-wire SPI uses 9 bits per word.
		bits = 9
	} else if err := dc.Out(gpio.Low); err != nil {
		return nil, err
	}
	if err := s.DevParams(3300000, spi.Mode0, bits); err != nil {
		return nil, err
	}
	return newDev(s, w, h, rotated, true, dc)
}

// NewI2C returns a Dev object that communicates over I²C to a SSD1306 display
// controller.
//
// If rotated, turns the display by 180°
func NewI2C(i i2c.Bus, w, h int, rotated bool) (*Dev, error) {
	// Maximum clock speed is 1/2.5µs = 400KHz.
	return newDev(&i2c.Dev{Bus: i, Addr: 0x3C}, w, h, rotated, false, nil)
}

// newDev is the common initialization code that is independent of the bus
// being used.
func newDev(c conn.Conn, w, h int, rotated, usingSPI bool, dc gpio.PinOut) (*Dev, error) {
	if w < 8 || w > 128 || w&7 != 0 {
		return nil, fmt.Errorf("ssd1306: invalid width %d", w)
	}
	if h < 8 || h > 64 || h&7 != 0 {
		return nil, fmt.Errorf("ssd1306: invalid height %d", h)
	}

	nbPages := h / 8
	pageSize := (w*h/8 + 7) / 8
	d := &Dev{
		c:      c,
		spi:    usingSPI,
		dc:     dc,
		w:      uint8(w),
		h:      uint8(h),
		buffer: make([]byte, nbPages*pageSize),
		// Mark scrolling as true, as a way to hack that the screen must be redrawn
		// on first Write() call. In fact, the screen *could* be scrolling and we
		// need to handle that.
		scrolling: true,
	}

	// Set COM output scan direction; C0 means normal; C8 means reversed
	comScan := byte(0xC8)
	// See page 40.
	columnAddr := byte(0xA1)
	if rotated {
		// Change order both horizontally and vertically.
		comScan = 0xC0
		columnAddr = byte(0xA0)
	}
	// Initialize the device by fully resetting all values.
	// Page 64 has the full recommended flow.
	// Page 28 lists all the commands.
	// Some values come from the DM-OLED096 datasheet p15.
	init := []byte{
		0xAE,       // Display off
		0xD3, 0x00, // Set display offset; 0
		0x40,       // Start display start line; 0
		columnAddr, // Set segment remap; RESET is column 127.
		comScan,    //
		0xDA, 0x12, // Set COM pins hardware configuration; see page 40
		0x81, 0xff, // Set max contrast
		0xA4,       // Set display to use GDDRAM content
		0xA6,       // Set normal display (0xA7 for inverted 0=lit, 1=dark)
		0xD5, 0x80, // Set osc frequency and divide ratio; power on reset value is 0x3F.
		0x8D, 0x14, // Enable charge pump regulator; page 62
		0xD9, 0xf1, // Set pre-charge period; from adafruit driver
		0xDB, 0x40, // Set Vcomh deselect level; page 32
		0x20, 0x00, // Set memory addressing mode to horizontal
		0xB0,                // Set page start address
		0x2E,                // Deactivate scroll
		0x00,                // Set column offset (lower nibble)
		0x10,                // Set column offset (higher nibble)
		0xA8, byte(d.h - 1), // Set multiplex ratio (number of lines to display)
		0xAF, // Display on
	}
	if err := d.sendCommand(init); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dev) String() string {
	if d.spi {
		return fmt.Sprintf("ssd1360.Dev{%s, %s, %dx%d}", d.c, d.dc, d.w, d.h)
	}
	return fmt.Sprintf("ssd1360.Dev{%s, %dx%d}", d.c, d.w, d.h)
}

// ColorModel implements devices.Display.
//
// It is a one bit color model, as implemented by image1bit.Bit.
func (d *Dev) ColorModel() color.Model {
	return image1bit.BitModel
}

// Bounds implements devices.Display. Min is guaranteed to be {0, 0}.
func (d *Dev) Bounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{X: int(d.w), Y: int(d.h)}}
}

// Draw implements devices.Display.
//
// It discards any failure.
func (d *Dev) Draw(r image.Rectangle, src image.Image, sp image.Point) {
	srcR := src.Bounds()
	// r is the destination image.Rectangle in d.buffer.
	r = r.Intersect(d.Bounds())
	// srcR is the image.Rectangle subset to be used from src.
	srcR.Min = srcR.Min.Add(sp)
	if dX := r.Dx(); dX < srcR.Dx() {
		srcR.Max.X = srcR.Min.X + dX
	}
	if dY := r.Dy(); dY < srcR.Dy() {
		srcR.Max.Y = srcR.Min.Y + dY
	}

	// delta is the difference between coordinate in src and d.buffer.
	delta := r.Min.Sub(srcR.Min)

	// TODO(maruel): Calculate delta by finding the smallest diffing rectangle
	// via brute force.
	startPage := uint8(0)
	endPage := d.h / 8
	startCol := uint8(0)
	endCol := d.w
	if img, ok := src.(*image1bit.Image); ok {
		if srcR.Min.X == 0 && srcR.Dx() == int(d.w) && srcR.Min.Y == 0 && srcR.Dy() == int(d.h) {
			// Exact size, full frame, image1bit encoding: fast path.
			copy(d.buffer, img.Buf)
		} else {
			// TODO(maruel): Optimize: do the non-8 horizontal lines first, then
			// 8-high block, then trailer horizontal band.
			for sY := srcR.Min.Y; sY < srcR.Max.Y; sY++ {
				destY := sY + delta.Y
				shift := uint8(destY & 7)
				mask := uint8(1 << shift)
				rY := (destY / 8) * int(d.w)
				for sX := srcR.Min.X; sX < srcR.Max.X; sX++ {
					x := rY + sX + delta.X
					d.buffer[x] = (d.buffer[x] &^ mask) | (img.Buf[sY/8*img.W+sX] & mask)
				}
			}
		}
	} else {
		// TODO(maruel): Optimize: do the non-8 horizontal lines first, then 8-high
		// block, then trailer horizontal band.
		for sY := srcR.Min.Y; sY < srcR.Max.Y; sY++ {
			destY := sY + delta.Y
			shift := uint8(destY & 7)
			mask := ^uint8(1 << shift)
			rY := (destY / 8) * int(d.w)
			for sX := srcR.Min.X; sX < srcR.Max.X; sX++ {
				x := rY + sX + delta.X
				d.buffer[x] = (d.buffer[x] & mask) | (colorToBit(src.At(sX, sY)) << shift)
			}
		}
	}
	if err := d.drawInternal(startPage, endPage, startCol, endCol); err != nil {
		log.Printf("ssd1306: Draw failed: %v", err)
	}
}

// Write writes a buffer of pixels to the display.
//
// The format is unsual as each byte represent 8 vertical pixels at a time. The
// format is horizontal bands of 8 pixels high.
func (d *Dev) Write(pixels []byte) (int, error) {
	if len(pixels) != len(d.buffer) {
		return 0, fmt.Errorf("ssd1306: invalid pixel stream length; expected %d bytes, got %d bytes", len(d.buffer), len(pixels))
	}

	startPage := uint8(0)
	endPage := d.h / 8
	startCol := uint8(0)
	endCol := d.w
	if d.scrolling {
		// Painting disable scrolling but if scrolling was enabled, this requires a
		// full screen redraw.
		d.scrolling = false
	} else {
		/*
				// Calculate the smallest square that need to be sent.
				for ; startPage <= endPage; startPage++ {
					chunk := pixels[d.pageSize*startPage : d.pageSize*(startPage+1)]
					if !bytes.Equal(d.pages[startPage], chunk) {
						break
					}
				}
				for ; endPage >= startPage; endPage-- {
					chunk := pixels[d.pageSize*endPage : d.pageSize*(endPage+1)]
					if !bytes.Equal(d.pages[endPage], chunk) {
						break
					}
				}
				if startPage > endPage {
					// Early exit, the image is exactly the same.
					goto end
				}
				for ; startCol <= endCol; startCol++ {
					// Compare 8 vertical pixels at a time.
					for i := startPage; i <= endPage; i++ {
						if d.pages[i][startCol] != pixels[d.pageSize*i+startCol] {
							goto diffStart
						}
					}
				}
			diffStart:
				for ; endCol >= startCol; endCol-- {
					// Compare 8 vertical pixels at a time.
					for i := startPage; i <= endPage; i++ {
						if d.pages[i][startCol] != pixels[d.pageSize*i+startCol] {
							goto diffEnd
						}
					}
				}
			diffEnd:
		*/
	}
	copy(d.buffer, pixels)
	if err := d.drawInternal(startPage, endPage, startCol, endCol); err != nil {
		return 0, err
	}
	return len(pixels), nil
}

// drawInternal sends image data to the controller.
func (d *Dev) drawInternal(startPage, endPage, startCol, endCol uint8) error {
	log.Printf("%s.drawInternal(%d, %d, %d, %d)", d, startPage, endPage, startCol, endCol)
	// The following commands should not be needed, but then if the SSD1306 gets
	// out of sync for some reason the display ends up messed-up. Given the small
	// overhead compared to sending all the data might as well reset things a
	// bit.
	cmd := []byte{
		0xB0,       // Set page start addr just in case
		0x00, 0x10, // Set column start addr, lower & upper nibble
		0x20, 0x00, // Ensure addressing mode is horizontal
		0x21, startCol, endCol - 1, // Set column address (Width)
		0x22, startPage, endPage - 1, // Set page address (Pages)
	}
	if err := d.sendCommand(cmd); err != nil {
		return err
	}

	// Write the subset of the data as needed.
	pageSize := (int(d.w)*int(d.h/8) + 7) / 8
	return d.sendData(d.buffer[int(startPage)*pageSize+int(startCol) : int(endPage-1)*pageSize+int(endCol)])
}

// Scroll scrolls an horizontal band.
//
// endLine is exclusive.
//
// If endLine is -1, use the rest of the screen.
func (d *Dev) Scroll(o Orientation, rate FrameRate, startLine, endLine int) error {
	if endLine == -1 {
		endLine = int(d.h)
	}
	if startLine >= endLine {
		return fmt.Errorf("startLine (%d) must be lower than endLine (%d)", startLine, endLine)
	}
	if startLine&7 != 0 || startLine < 0 || startLine >= int(d.h) {
		return fmt.Errorf("invalid startLine %d", startLine)
	}
	if endLine&7 != 0 || endLine < 0 || endLine > int(d.h) {
		return fmt.Errorf("invalid endLine %d", endLine)
	}

	startPage := uint8(startLine / 8)
	endPage := uint8(endLine / 8)
	d.scrolling = true
	if o == Left || o == Right {
		// page 28
		// STOP, <op>, dummy, <start page>, <rate>,  <end page>, <dummy>, <dummy>, <ENABLE>
		return d.sendCommand([]byte{0x2E, byte(o), 0x00, startPage, byte(rate), endPage - 1, 0x00, 0xFF, 0x2F})
	}
	// page 29
	// STOP, <op>, dummy, <start page>, <rate>,  <end page>, <offset>, <ENABLE>
	// page 30: 0xA3 permits to set rows for scroll area.
	return d.sendCommand([]byte{0x2E, byte(o), 0x00, startPage, byte(rate), endPage - 1, 0x01, 0x2F})
}

// StopScroll stops any scrolling previously set and resets the screen.
func (d *Dev) StopScroll() error {
	return d.sendCommand([]byte{0x2E})
}

// SetContrast changes the screen contrast.
//
// Note: values other than 0xff do not seem useful...
func (d *Dev) SetContrast(level byte) error {
	return d.sendCommand([]byte{0x81, level})
}

// Enable or disable the display.
func (d *Dev) Enable(on bool) error {
	b := []byte{0xAE}
	if on {
		b[0] = 0xAF
	}
	return d.sendCommand(b)
}

// Invert the display (black on white vs white on black).
func (d *Dev) Invert(blackOnWhite bool) error {
	b := []byte{0xA6}
	if blackOnWhite {
		b[0] = 0xA7
	}
	return d.sendCommand(b)
}

//

func (d *Dev) sendData(c []byte) error {
	if d.spi {
		if d.dc == nil {
			// 3-wire SPI.
			return errors.New("ssd1306: 3-wire SPI mode is not yet implemented")
		}
		// 4-wire SPI.
		if err := d.dc.Out(gpio.High); err != nil {
			return err
		}
		return d.c.Tx(c, nil)
	}
	return d.c.Tx(append([]byte{i2cData}, c...), nil)
}

func (d *Dev) sendCommand(c []byte) error {
	if d.spi {
		if d.dc == nil {
			// 3-wire SPI.
			return errors.New("ssd1306: 3-wire SPI mode is not yet implemented")
		}
		// 4-wire SPI.
		if err := d.dc.Out(gpio.Low); err != nil {
			return err
		}
		return d.c.Tx(c, nil)
	}
	return d.c.Tx(append([]byte{i2cCmd}, c...), nil)
}

const (
	i2cCmd  = 0x00 // I²C transaction has stream of command bytes
	i2cData = 0x40 // I²C transaction has stream of data bytes
)

func colorToBit(c color.Color) byte {
	r, g, b, a := c.RGBA()
	if (r|g|b) >= 0x8000 && a >= 0x4000 {
		return 1
	}
	return 0
}

var _ devices.Display = &Dev{}
