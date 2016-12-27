// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gpio

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func ExamplePinIn() {
	//p := gpioreg.ByNumber(6)
	var p PinIn
	if err := p.In(PullDown, RisingEdge); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s is %s\n", p, p.Read())
	for p.WaitForEdge(-1) {
		fmt.Printf("%s went %s\n", p, High)
	}
}

func ExamplePinOut() {
	//p := gpioreg.ByNumber(6)
	var p PinOut
	if err := p.Out(High); err != nil {
		log.Fatal(err)
	}
}

func ExamplePinStreamReader_ReadStream() {
	// Read one second of sample at 1ms resolution and print the values read.
	//p := gpioreg.ByNumber(6)
	var p PinIn
	b := make(Bits, 1000/8)
	p.(PinStreamReader).ReadStream(PullDown, time.Millisecond, b)
	for i := range b {
		for j := 0; j < 8; j++ {
			fmt.Printf("%s\n", Level(b[i]&(1<<uint(j)) != 0))
		}
	}
}

/*
func ExamplePinStreamReader() {
	// Continuously read samples at 100ms resolution. Create two buffers of 800ms.
	res := 100 * time.Millisecond
	b := []BitStream{{Res: res, Bits: make([]byte, 1)}, {Res: res, Bits: make([]byte, 1)}}
	p := gpioreg.ByNumber(6).(PinStreamReader)
	p.EnqueueReadStream(&b[0])
	for x := 1; ; x = (x + 1) & 1 {
		p.EnqueueReadStream(&b[x])
		// Wait
		for i := range b[x].Bits {
			for j := 7; j >= 0; j-- {
				fmt.Printf("%s\n", Level(b[x].Bits[i]&(1<<uint(j)) != 0))
			}
		}
	}
}
*/

func TestStrings(t *testing.T) {
	if Low.String() != "Low" || High.String() != "High" {
		t.Fail()
	}
	if Float.String() != "Float" || Pull(100).String() != "Pull(100)" {
		t.Fail()
	}
	if NoEdge.String() != "NoEdge" || Edge(100).String() != "Edge(100)" {
		t.Fail()
	}
}

func TestDuty(t *testing.T) {
	data := []struct {
		d        Duty
		expected string
	}{
		{0, "0%"},
		{1, "0%"},
		{DutyMax / 200, "0%"},
		{DutyMax/100 - 1, "1%"},
		{DutyMax / 100, "1%"},
		{DutyMax, "100%"},
		{DutyMax - 1, "100%"},
		{DutyHalf, "50%"},
		{DutyHalf + 1, "50%"},
		{DutyHalf - 1, "50%"},
		{DutyHalf + DutyMax/100, "51%"},
		{DutyHalf - DutyMax/100, "49%"},
	}
	for i, line := range data {
		if actual := line.d.String(); actual != line.expected {
			t.Fatalf("line %d: Duty(%d).String() == %q, expected %q", i, line.d, actual, line.expected)
		}
	}
}

func TestInvalid(t *testing.T) {
	if INVALID.String() != "INVALID" || INVALID.Name() != "INVALID" || INVALID.Number() != -1 || INVALID.Function() != "" {
		t.Fail()
	}
	if INVALID.In(Float, NoEdge) != errInvalidPin || INVALID.Read() != Low || INVALID.WaitForEdge(time.Minute) || INVALID.Pull() != PullNoChange {
		t.Fail()
	}
	if INVALID.Out(Low) != errInvalidPin {
		t.Fail()
	}
}
