// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// allwinnersmoketest verifies that allwinner specific functionality work.
//
// This test assumes GPIO pins are connected together. The exact ones depends
// on the actual board.
package allwinnersmoketest

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/host/allwinner"
	"periph.io/x/periph/host/chip"
	"periph.io/x/periph/host/pine64"
)

type SmokeTest struct {
	// start is to display the delta in µs.
	start time.Time
}

func (s *SmokeTest) Name() string {
	return "allwinner"
}

func (s *SmokeTest) Description() string {
	return "Tests advanced Allwinner functionality"
}

func (s *SmokeTest) Run(args []string) error {
	if !allwinner.Present() {
		return errors.New("this smoke test can only be used on a Allwinner based host")
	}
	f := flag.NewFlagSet("allwinner", flag.ExitOnError)
	f.Parse(args)
	if f.NArg() != 0 {
		return errors.New("unsupported flags")
	}

	start := time.Now()
	var pwm *loggingPin
	var other *loggingPin
	if chip.Present() {
		pwm = &loggingPin{allwinner.PB2, start}
		other = &loggingPin{allwinner.PB3, start}
	} else if pine64.Present() {
		//pwm = &loggingPin{allwinner.PD22}
		return errors.New("implement and test for pine64")
	} else {
		return errors.New("implement and test for this host")
	}
	if err := ensureConnectivity(pwm, other); err != nil {
		return err
	}
	return s.testPWM(pwm, other)
}

// Returns a channel that will return one bool, true if a edge was detected,
// false otherwise.
func (s *SmokeTest) waitForEdge(p gpio.PinIO) <-chan bool {
	c := make(chan bool)
	// A timeout inherently makes this test flaky but there's a inherent
	// assumption that the CPU edge trigger wakes up this process within a
	// reasonable amount of time; in term of latency.
	go func() {
		b := p.WaitForEdge(time.Second)
		// Author note: the test intentionally doesn't call p.Read() to test that
		// reading is not necessary.
		fmt.Printf("    %s -> WaitForEdge(%s) -> %t\n", since(s.start), p, b)
		c <- b
	}()
	return c
}

func (s *SmokeTest) testPWM(pwm, other *loggingPin) error {
	fmt.Printf("- Testing PWM\n")
	if err := other.In(gpio.PullDown, gpio.BothEdges); err != nil {
		return err
	}
	time.Sleep(time.Microsecond)
	if err := pwm.PWM(32000, time.Millisecond); err != nil {
		return err
	}
	if err := other.PWM(32000, time.Millisecond); err == nil {
		return fmt.Errorf("%s shouldn't be supported in PWM mode", other)
	}
	return nil
}

//

func printPin(p gpio.PinIO) {
	fmt.Printf("- %s: %s", p, p.Function())
	if r, ok := p.(gpio.RealPin); ok {
		fmt.Printf("  alias for %s", r.Real())
	}
	fmt.Print("\n")
}

// since returns time in µs since the test start.
func since(start time.Time) string {
	µs := (time.Since(start) + time.Microsecond/2) / time.Microsecond
	ms := µs / 1000
	µs %= 1000
	return fmt.Sprintf("%3d.%03dms", ms, µs)
}

// loggingPin logs when its state changes.
type loggingPin struct {
	*allwinner.Pin
	start time.Time
}

func (p *loggingPin) In(pull gpio.Pull, edge gpio.Edge) error {
	fmt.Printf("  %s %s.In(%s, %s)\n", since(p.start), p, pull, edge)
	return p.Pin.In(pull, edge)
}

func (p *loggingPin) Out(l gpio.Level) error {
	fmt.Printf("  %s %s.Out(%s)\n", since(p.start), p, l)
	return p.Pin.Out(l)
}

func (p *loggingPin) PWM(duty gpio.Duty, period time.Duration) error {
	fmt.Printf("  %s %s.PWM(%s, %s)\n", since(p.start), p, duty, period)
	return p.Pin.PWM(duty, period)
}

// ensureConnectivity makes sure they are connected together.
func ensureConnectivity(p1, p2 *loggingPin) error {
	if err := p1.In(gpio.PullDown, gpio.NoEdge); err != nil {
		return err
	}
	if err := p2.In(gpio.PullDown, gpio.NoEdge); err != nil {
		return err
	}
	time.Sleep(time.Microsecond)
	if p1.Read() != gpio.Low {
		return fmt.Errorf("unexpected %s value; expected low", p1)
	}
	if p2.Read() != gpio.Low {
		return fmt.Errorf("unexpected %s value; expected low", p2)
	}
	if err := p2.In(gpio.PullUp, gpio.NoEdge); err != nil {
		return err
	}
	time.Sleep(time.Microsecond)
	if p1.Read() != gpio.High {
		return fmt.Errorf("unexpected %s value; expected high", p1)
	}
	if err := p1.In(gpio.Float, gpio.NoEdge); err != nil {
		return err
	}
	if err := p2.In(gpio.Float, gpio.NoEdge); err != nil {
		return err
	}
	return nil
}
