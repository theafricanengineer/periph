// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bcm283x

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var clockMemory *clockMap

const (
	clk19dot2MHz = 19200000
	clk500MHz    = 500000000
)

const (
	// 31:24 password
	clockPasswdCtl clockCtl = 0x5A << 24 // PASSWD
	// 23:11 reserved
	clockMashMask clockCtl = 3 << 9 // MASH
	clockMash0    clockCtl = 0 << 9 // src_freq / divI  (ignores divF)
	clockMash1    clockCtl = 1 << 9
	clockMash2    clockCtl = 2 << 9
	clockMash3    clockCtl = 3 << 9 // will cause higher spread
	clockFlip     clockCtl = 1 << 8 // FLIP
	clockBusy     clockCtl = 1 << 7 // BUSY
	// 6 reserved
	clockKill          clockCtl = 1 << 5   // KILL
	clockEnable        clockCtl = 1 << 4   // ENAB
	clockSrcMask       clockCtl = 0xF << 0 // SRC
	clockSrcGND        clockCtl = 0        // 0Hz
	clockSrc19dot2MHz  clockCtl = 1        // 19.2MHz
	clockSrcTestDebug0 clockCtl = 2        // 0Hz
	clockSrcTestDebug1 clockCtl = 3        // 0Hz
	clockSrcPLLA       clockCtl = 4        // 0Hz
	clockSrcPLLC       clockCtl = 5        // 1000MHz (changes with overclock settings)
	clockSrcPLLD       clockCtl = 6        // 500MHz
	clockSrcHDMI       clockCtl = 7        // 216MHz; may be disabled
	// 8-15 == GND.
)

// clockCtl controls the clock properties.
//
// It must not be changed while busy is set or a glitch may occur.
//
// Page 107
type clockCtl uint32

func (c clockCtl) GoString() string {
	var out []string
	if c&0xFF000000 == clockPasswdCtl {
		c &^= 0xFF000000
		out = append(out, "PWD")
	}
	switch c & clockMashMask {
	case clockMash1:
		out = append(out, "Mash1")
	case clockMash2:
		out = append(out, "Mash2")
	case clockMash3:
		out = append(out, "Mash3")
	default:
	}
	c &^= clockMashMask
	if c&clockFlip != 0 {
		out = append(out, "Flip")
		c &^= clockFlip
	}
	if c&clockBusy != 0 {
		out = append(out, "Busy")
		c &^= clockBusy
	}
	if c&clockKill != 0 {
		out = append(out, "Kill")
		c &^= clockKill
	}
	if c&clockEnable != 0 {
		out = append(out, "Enable")
		c &^= clockEnable
	}
	switch x := c & clockSrcMask; x {
	case clockSrcGND:
		out = append(out, "GND(0Hz)")
	case clockSrc19dot2MHz:
		out = append(out, "19.2MHz")
	case clockSrcTestDebug0:
		out = append(out, "Debug0(0Hz)")
	case clockSrcTestDebug1:
		out = append(out, "Debug1(0Hz)")
	case clockSrcPLLA:
		out = append(out, "PLLA(0Hz)")
	case clockSrcPLLC:
		out = append(out, "PLLD(1000MHz)")
	case clockSrcPLLD:
		out = append(out, "PLLD(500MHz)")
	case clockSrcHDMI:
		out = append(out, "HDMI(216MHz)")
	default:
		out = append(out, fmt.Sprintf("GND(%d)", x))
	}
	c &^= clockSrcMask
	if c != 0 {
		out = append(out, fmt.Sprintf("clockCtl(%d)", c))
	}
	return strings.Join(out, "|")
}

const (
	// 31:24 password
	clockPasswdDiv clockDiv = 0x5A << 24 // PASSWD
	// Integer part of the divisor
	clockDiviShift          = 12
	clockDiviMax            = (1 << 12) - 1
	clockDiviMask  clockDiv = clockDiviMax << clockDiviShift // DIVI
	// Fractional part of the divisor
	clockDivfMask clockDiv = (1 << 12) - 1 // DIVF
)

// clockDiv is a 12.12 fixed point value.
//
// The fractional part generates a significan amount of noise so it is
// preferable to not use it.
//
// Page 108
type clockDiv uint32

func (c clockDiv) GoString() string {
	i := (c & clockDiviMask) >> clockDiviShift
	c &^= clockDiviMask
	if c == 0 {
		return fmt.Sprintf("%d.0", i)
	}
	return fmt.Sprintf("%d.(%d/%d)", i, c, clockDiviMax)
}

// clock is a pair of clockCtl / clockDiv.
//
// It can be set to one of the sources: clockSrc19dot2MHz(19.2MHz) and
// clockSrcPLLD(500Mhz), then divided to a value to get the resulting clock.
// Per spec the resulting frequency should be under 25Mhz.
type clock struct {
	ctl clockCtl
	div clockDiv
}

// findDivisorExact finds the divisors x and y to reduce src to desired hz.
//
// Returns divisors x, y. Returns 0, 0 if no exact match is found. Favorizes
// high x over y. This means that the function is slower than it could be, but
// results in more stable clock.
func findDivisorExact(srcHz, desiredHz uint64, x, y int) (int, int) {
	if x < y {
		panic(fmt.Errorf("%d must be >= to %d", x, y))
	}
	for j := 1; j <= y; j++ {
		if srcHz%uint64(j) != 0 {
			continue
		}
		d := srcHz / uint64(j)
		if d < desiredHz {
			break
		}
		for i := j; i <= x; i++ {
			// Doing an early exit is actually slower on x64. Needs to be
			// confirmed on ARM.
			if d%uint64(i) == 0 && d/uint64(i) == desiredHz {
				return i, j
			}
		}
	}
	return 0, 0
}

// findDivisor finds the best divisors x and y to reduce src to desired hz.
//
// Returns divisors x, y, actual frequency, error. The actual selected
// frequency may be largely oversampled.
func findDivisor(srcHz, desiredHz uint64, x, y int) (int, int, uint64, uint64) {
	if m, n := findDivisorExact(srcHz, desiredHz, x, y); m != 0 {
		return m, n, desiredHz, 0
	}
	// Allowed oversampling depends on the desiredHz. Cap oversampling because
	// oversampling at 10x in the 1Mhz range becomes unreasonable in term of
	// memory usage.
	for i := uint64(2); ; i++ {
		d := i * desiredHz
		if d > 100000 && i > 10 {
			break
		}
		if m, n := findDivisorExact(srcHz, d, x, y); m != 0 {
			return m, n, d, 0
		}
	}
	// There's no exact match, even with oversampling. That means that we need to
	// select a value with errors. Oversample by 2x to reduce the relative error
	// a bit. Multiply both srcHz and desired by 100x to include additional small
	// error detection.
	desiredHz *= 200
	srcHz *= 100
	minErr := uint64(0xFFFFFFFFFFFFFFF)
	m := 0
	n := 0
	selected := uint64(0)
	for i := 1; i <= x; i++ {
		maxY := y
		if maxY > i {
			maxY = i
		}
		for j := 1; j <= maxY; j++ {
			actual := (srcHz / uint64(i)) / uint64(j)
			err := (actual - desiredHz)
			if err < 0 {
				err = -err
			}
			if minErr > err {
				minErr = err
				selected = actual
				m = i
				n = j
			}
		}
	}
	return m, n, selected / 100, minErr / 100
}

// calcSource chose the best source to get the exact desired clock.
//
// It calculates the clock source, the clock divisor and the wait cycles, if
// applicable. Wait cycles is 'div minus 1'.
func calcSource(hz uint64, maxDiv int) (clockCtl, int, int, uint64, error) {
	if hz > 25000000 {
		return 0, 0, 0, 0, fmt.Errorf("bcm283x-clock: desired frequency %dHz is too high", hz)
	}
	// http://elinux.org/BCM2835_datasheet_errata states that clockSrc19dot2MHz
	// is the cleanest clock source so try it first.
	x19, y19, actual19, rest19 := findDivisor(clk19dot2MHz, hz, clockDiviMax, maxDiv)
	if rest19 == 0 {
		return clockSrc19dot2MHz, x19, y19, actual19, nil
	}
	x500, y500, actual500, rest500 := findDivisor(clk500MHz, hz, clockDiviMax, maxDiv)
	if rest500 == 0 {
		return clockSrcPLLD, x500, y500, actual500, nil
	}
	// No exact match. Choose the one with the lowest (absolute) error.
	if rest19 < rest500 {
		return clockSrc19dot2MHz, x19, y19, actual19, nil
	}
	return clockSrcPLLD, x500, y500, actual500, nil
}

// set changes the clock frequency to the desired value or the closest one
// otherwise.
//
// 0 means disabled.
//
// Returns the actual clock used and divisor.
func (c *clock) set(hz uint64, maxOversample int) (uint64, int, error) {
	if hz == 0 {
		c.ctl = clockPasswdCtl | clockKill
		for c.ctl&clockBusy != 0 {
		}
		return 0, 0, nil
	}
	ctl, div, div2, actual, err := calcSource(hz, maxOversample)
	if err != nil {
		return 0, 0, err
	}
	return actual, div2, c.setRaw(ctl, div)
}

// setRaw sets the clock speed with the clock source and the divisor.
func (c *clock) setRaw(ctl clockCtl, div int) error {
	if div < 1 || div > clockDiviMax {
		return errors.New("invalid clock divisor")
	}
	if ctl != clockSrc19dot2MHz && ctl != clockSrcPLLD {
		return errors.New("invalid clock control")
	}
	// Stop the clock.
	// TODO(maruel): Do not stop the clock if the current clock rate is the one
	// desired.
	for c.ctl&clockBusy != 0 {
		c.ctl = clockPasswdCtl | clockKill
	}
	d := clockDiv(div << clockDiviShift)
	c.div = clockPasswdDiv | d
	Nanospin(10 * time.Nanosecond)
	// Page 107
	c.ctl = clockPasswdCtl | ctl
	Nanospin(10 * time.Nanosecond)
	c.ctl = clockPasswdCtl | ctl | clockEnable
	if c.div != d {
		return errors.New("can't write to clock divisor CPU register")
	}
	return nil
}

func (c *clock) GoString() string {
	return fmt.Sprintf("{%#v, %#v}", c.ctl, c.div)
}

// clockMap is the memory mapped clock registers.
//
// The clock #1 must not be touched since it is being used by the ethernet
// controller.
//
// Page 107 for gp0~gp2.
// https://scribd.com/doc/127599939/BCM2835-Audio-clocks for PCM/PWM.
type clockMap struct {
	reserved0 [0x70 / 4]uint32          //
	gp0       clock                     // CM_GP0CTL+CM_GP0DIV; 0x70-0x74 (125MHz max)
	gp1ctl    uint32                    // CM_GP1CTL+CM_GP1DIV; 0x78-0x7A must not use (used by ethernet)
	gp1div    uint32                    // CM_GP1CTL+CM_GP1DIV; 0x78-0x7A must not use (used by ethernet)
	gp2       clock                     // CM_GP2CTL+CM_GP2DIV; 0x80-0x84 (125MHz max)
	reserved1 [(0x98 - 0x88) / 4]uint32 // 0x88-0x94
	pcm       clock                     // CM_PCMCTL+CM_PCMDIV 0x98-0x9C
	pwm       clock                     // CM_PWMCTL+CM_PWMDIV 0xA0-0xA4
}

func (c *clockMap) GoString() string {
	return fmt.Sprintf("{\n  gp0: %#v,\n  gp1: {%#v, %#v}\n  gp2: %#v,\n  pcm: %#v,\n  pwm: %#v,\n}", &c.gp0, clockCtl(c.gp1ctl), clockDiv(c.gp1div), &c.gp2, &c.pcm, &c.pwm)
}
