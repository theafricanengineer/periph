// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gpio defines digital pins.
//
// The GPIO pins are described in their logical functionality, not in their
// physical position.
//
// While all GPIO implementations are expected to implement PinIO, they may
// expose more specific functionality like PWM(), Clock(), etc.
//
// If you are looking to identify a GPIO pin from its position on a board
// header, look for periph/host/headers.
package gpio

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"periph.io/x/periph/conn/pin"
)

// Interfaces

// INVALID implements PinIO and fails on all access.
var INVALID PinIO

// Level is the level of the pin: Low or High.
type Level bool

const (
	// Low represents 0v.
	Low Level = false
	// High represents Vin, generally 3.3v or 5v.
	High Level = true
)

func (l Level) String() string {
	if l == Low {
		return "Low"
	}
	return "High"
}

// Pull specifies the internal pull-up or pull-down for a pin set as input.
type Pull uint8

// Acceptable pull values.
const (
	Float        Pull = 0 // Let the input float
	PullDown     Pull = 1 // Apply pull-down
	PullUp       Pull = 2 // Apply pull-up
	PullNoChange Pull = 3 // Do not change the previous pull resistor setting or an unknown value
)

const pullName = "FloatPullDownPullUpPullNoChange"

var pullIndex = [...]uint8{0, 5, 13, 19, 31}

func (i Pull) String() string {
	if i >= Pull(len(pullIndex)-1) {
		return fmt.Sprintf("Pull(%d)", i)
	}
	return pullName[pullIndex[i]:pullIndex[i+1]]
}

// Edge specifies if an input pin should have edge detection enabled.
//
// Only enable it when needed, since this causes system interrupts.
type Edge int

// Acceptable edge detection values.
const (
	NoEdge      Edge = 0
	RisingEdge  Edge = 1
	FallingEdge Edge = 2
	BothEdges   Edge = 3
)

const edgeName = "NoEdgeRisingEdgeFallingEdgeBothEdges"

var edgeIndex = [...]uint8{0, 6, 16, 27, 36}

func (i Edge) String() string {
	if i >= Edge(len(edgeIndex)-1) {
		return fmt.Sprintf("Edge(%d)", i)
	}
	return edgeName[edgeIndex[i]:edgeIndex[i+1]]
}

const (
	// DutyMax is a duty cycle of 100%.
	DutyMax Duty = 65535
	// DutyHalf is a 50% duty PWM, which boils down to a normal clock.
	DutyHalf Duty = DutyMax / 2
)

// Duty is the duty cycle for a PWM. It varies between 0 and DutyMax.
//
// It is the more efficient equivalent of a StreamEdges with two Edges.
type Duty uint16

func (d Duty) String() string {
	// TODO(maruel): Implement one fractional number.
	return fmt.Sprintf("%d%%", (uint32(d)+50)/(uint32(DutyMax)/100))
}

// ParseDuty parses a string and converts it to a Duty value.
func ParseDuty(s string) (Duty, error) {
	percent := strings.HasSuffix(s, "%")
	if percent {
		s = s[:len(s)-1]
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	if percent {
		// TODO(maruel): Add support for fractional number.
		if i < 0 {
			return 0, errors.New("duty must be >= 0%")
		}
		if i > 100 {
			return 0, errors.New("duty must be <= 100%")
		}
		return Duty((int(i)*int(DutyMax) + 49) / 100), nil
	}
	if i < 0 {
		return 0, errors.New("duty must be >= 0")
	}
	if i > int64(DutyMax) {
		return 0, fmt.Errorf("duty must be <= %d", DutyMax)
	}
	return Duty(i), nil
}

// PWMer exposes hardware PWM.
//
// It can also be implemented as PinStreamer depending on the hardware
// configuration.
type PWMer interface {
	// PWM sets the PWM output on supported pins.
	//
	// To use as a general purpose clock, set duty to DutyHalf. Some pins may
	// only support DutyHalf and no other value.
	//
	// Using 0 as period will use the default value the default value as
	// supported/preferred by the pin.
	PWM(duty Duty, period time.Duration) error
}

// Stream is the interface accepted by PinStreamer.
//
// Only implementations in package stream are supported.
type Stream interface {
	// Resolution of the binary stream.
	Resolution() time.Duration
	// Duration of the binary stream.
	Duration() time.Duration
}

// Bits is a densely packed bitstream. The exact format is LSB, that is, bit
// 0 is sent first, up to bit 7.
//
// A more natural format would be MSB but it's a pain to iterate through.
//
// The stream is required to be a multiple of 8 samples.
//
// This type exists becase []bool isn't compressed. :/
type Bits []byte

// PinStreamer is an optional interface for GPIO pin that supports streaming
// arbitrary binary waveforms.
//
// This is useful to bitbang a protocol.
//
// The GPIO pin will try its best to run the following according to other
// limiting factors that are implementation specific.
//
// On bcm2383x, streaming bits takes a large amount of data but any pin can
// stream arbitrary data.
//
// On allwinner, 8 consecutive pins must be controlled simultaneously (e.g. PB0
// to PB7, PB8 to PB15) but the memory usage is 8 times lower than on bcm283x.
//
// See the driver's implementation documentation for more details.
type PinStreamer interface {
	// Stream immediately streams a bit stream. The function may block if the
	// amount of data to stream is significantly larger than the internal buffer.
	//
	// This has the effect of putting the pin in output mode.
	Stream(s Stream) error

	// EnqueueStream enqueues the stream to start at the specified time and
	// immediately return.
	//
	// The caller can continue enqueuing more data for a glitch-free operation.
	// started, if specified, is activated when the data is started.
	//EnqueueStream(s Stream, started <-chan bool) error
}

// PinStreamReader is an optional interface for GPIO pin that that read at a
// constant pace.
//
// The resulting stream read is stored in b.
//
// This is useful to bitbang a protocol or create a binary oscilloscope.
type PinStreamReader interface {
	// ReadStream reads a stream of data.
	//
	// This has the effect of putting the pin in input mode.
	ReadStream(pull Pull, resolution time.Duration, b Bits) error
	// EnqueueReadStream enqueues a buffer to read at this specific time.
	//EnqueueReadStream(pull Pull, resolution time.Duration, b Bits) error
}

// PinIn is an input GPIO pin.
//
// It may optionally support internal pull resistor and edge based triggering.
type PinIn interface {
	pin.Pin
	// Close is a way to signal that the user doesn't care about the pin anymore.
	//Close() error
	// In setups a pin as an input.
	//
	// If WaitForEdge() is planned to be called, make sure to use one of the Edge
	// value. Otherwise, use None to not generated unneeded hardware interrupts.
	//
	// Calling In() will try to empty the accumulated edges but it cannot be 100%
	// reliable due to the OS (linux) and its driver. It is possible that on a
	// gpio that is as input, doing a quick Out(), In() may return an edge that
	// occurred before the Out() call.
	In(pull Pull, edge Edge) error
	// Read return the current pin level.
	//
	// Behavior is undefined if In() wasn't used before.
	//
	// In some rare case, it is possible that Read() fails silently. This happens
	// if another process on the host messes up with the pin after In() was
	// called. In this case, call In() again.
	Read() Level
	// WaitForEdge() waits for the next edge or immediately return if an edge
	// occurred since the last call.
	//
	// Only waits for the kind of edge as specified in a previous In() call.
	// Behavior is undefined if In() with a value other than None wasn't called
	// before.
	//
	// Returns true if an edge was detected during or before this call. Return
	// false if the timeout occurred or In() was called while waiting, causing the
	// function to exit.
	//
	// Multiple edges may or may not accumulate between two calls to
	// WaitForEdge(). The behavior in this case is undefined and is OS driver
	// specific.
	//
	// It is not required to call Read() to reset the edge detection.
	//
	// Specify -1 to effectively disable timeout.
	WaitForEdge(timeout time.Duration) bool
	// Pull returns the current internal pull resistor setting if the pin is set
	// currently as an input pin and input pull resistor is supported by the
	// driver.
	//
	// Returns PullNoChange if the value cannot be read.
	Pull() Pull
}

// PinOut is an output GPIO pin.
type PinOut interface {
	pin.Pin
	// Close is a way to signal that the user doesn't care about the pin anymore.
	//Close() error
	// Out sets a pin as output if it wasn't already and sets the initial value.
	//
	// After the initial call to ensure that the pin has been set as output, it
	// is generally safe to ignore the error returned.
	//
	// Out() tries to empty the accumulated edges detected if the gpio was
	// previously set as input but this is not 100% guaranteed due to the OS.
	Out(l Level) error
}

// PinIO is a GPIO pin that supports both input and output.
//
// It may fail at either input and or output for unidirectional pin.
//
// The GPIO pin may optionally support more interfaces, like DefaultPuller,
// PinStreamer and PinStreamReader.
type PinIO interface {
	pin.Pin
	// Close is a way to signal that the user doesn't care about the pin anymore.
	//Close() error
	// PinIn
	In(pull Pull, edge Edge) error
	Read() Level
	WaitForEdge(timeout time.Duration) bool
	Pull() Pull
	// PinOut
	Out(l Level) error
}

// DefaultPuller is optionally implemented to return the default pull at boot
// time. This is useful to determine if the pin is acceptable for operation
// with certain devices.
type DefaultPuller interface {
	// DefaultPull returns the pull that is initialized on CPU reset.
	DefaultPull() Pull
}

// RealPin is implemented by aliased pin and allows the retrieval of the real
// pin underlying an alias.
//
// The purpose of the RealPin is to be able to cleanly test whether an arbitrary
// gpio.PinIO returned by ByName is really an alias for another pin.
type RealPin interface {
	Real() PinIO // Real returns the real pin behind an Alias
}

//

// errInvalidPin is returned when trying to use INVALID.
var errInvalidPin = errors.New("gpio: invalid pin")

func init() {
	INVALID = invalidPin{}
}

// invalidPin implements PinIO for compatibility but fails on all access.
type invalidPin struct {
}

func (invalidPin) Number() int {
	return -1
}

func (invalidPin) Name() string {
	return "INVALID"
}

func (invalidPin) String() string {
	return "INVALID"
}

func (invalidPin) Function() string {
	return ""
}

func (invalidPin) In(Pull, Edge) error {
	return errInvalidPin
}

func (invalidPin) Read() Level {
	return Low
}

func (invalidPin) WaitForEdge(timeout time.Duration) bool {
	return false
}

func (invalidPin) Pull() Pull {
	return PullNoChange
}

func (invalidPin) Out(Level) error {
	return errInvalidPin
}

func (invalidPin) PWM(duty int) error {
	return errInvalidPin
}

// pinAlias implements an alias for a PinIO.
//
// pinAlias also implements the RealPin interface, which allows querying for
// the real pin under the alias.
type pinAlias struct {
	PinIO
	name   string
	number int
}

// String returns the alias name along the real pin's Name() in parenthesis, if
// known, else the real pin's number.
func (a *pinAlias) String() string {
	if a.PinIO == nil {
		return fmt.Sprintf("%s(%d)", a.name, a.number)
	}
	return fmt.Sprintf("%s(%s)", a.name, a.PinIO.Name())
}

// Name returns the pinAlias's name.
func (a *pinAlias) Name() string {
	return a.name
}

// Real returns the real pin behind the alias
func (a *pinAlias) Real() PinIO {
	return a.PinIO
}

var _ PinIn = INVALID
var _ PinOut = INVALID
var _ PinIO = INVALID
