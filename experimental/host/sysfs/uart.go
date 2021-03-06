// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package sysfs implements experimental sysfs support not yet in mainline.
package sysfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/experimental/conn/uart"
)

// EnumerateUART returns the available serial buses.
func EnumerateUART() ([]int, error) {
	// Do not use "/sys/class/tty/ttyS0/" as these are all owned by root.
	prefix := "/dev/ttyS"
	items, err := filepath.Glob(prefix + "*")
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		i, err := strconv.Atoi(item[len(prefix):])
		if err != nil {
			continue
		}
		out = append(out, i)
	}
	return out, nil
}

// UART is an open serial bus via sysfs.
//
// TODO(maruel): It's not yet implemented. Should probably defer to an already
// working library like https://github.com/tarm/serial
type UART struct {
	f         *os.File
	busNumber int
}

func newUART(busNumber int) (*UART, error) {
	// Use the devfs path for now.
	f, err := os.OpenFile(fmt.Sprintf("/dev/ttyS%d", busNumber), os.O_RDWR, os.ModeExclusive)
	if err != nil {
		return nil, err
	}
	u := &UART{f: f, busNumber: busNumber}
	return u, nil
}

// Close implements uart.ConnCloser.
func (u *UART) Close() error {
	err := u.f.Close()
	u.f = nil
	return err
}

func (u *UART) String() string {
	return "uart"
}

// Configure implements uart.Conn.
func (u *UART) Configure(stopBit uart.Stop, parity uart.Parity, bits int) error {
	return errors.New("sysfs-uart: not implemented")
}

// Write implements uart.Conn.
func (u *UART) Write(b []byte) (int, error) {
	return 0, errors.New("sysfs-uart: not implemented")
}

// Tx implements uart.Conn.
func (u *UART) Tx(w, r []byte) error {
	return errors.New("sysfs-uart: not implemented")
}

// Speed implements uart.Conn.
func (u *UART) Speed(hz int64) error {
	return errors.New("sysfs-uart: not implemented")
}

// RX implements uart.Pins.
func (u *UART) RX() gpio.PinIn {
	return gpio.INVALID
}

// TX implements uart.Pins.
func (u *UART) TX() gpio.PinOut {
	return gpio.INVALID
}

// RTS implements uart.Pins.
func (u *UART) RTS() gpio.PinIO {
	return gpio.INVALID
}

// CTS implements uart.Pins.
func (u *UART) CTS() gpio.PinIO {
	return gpio.INVALID
}

var _ uart.Conn = &UART{}

// driverUART implements periph.Driver.
type driverUART struct {
}

func (d *driverUART) String() string {
	return "sysfs-uart"
}

func (d *driverUART) Init() (bool, error) {
	return true, nil
}
