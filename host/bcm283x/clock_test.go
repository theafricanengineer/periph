// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bcm283x

import (
	"fmt"
	"testing"
)

func TestFindDivisorExact(t *testing.T) {
	t.Parallel()
	m, n := findDivisorExact(192000000, 216*7, clockDiviMax, dmaWaitcyclesMax+1)
	if m != 0 || n != 0 {
		t.Fatalf("%d != %d || %d != %d", m, 0, n, 0)
	}
}

func TestFindDivisor(t *testing.T) {
	t.Parallel()
	// clockDiviMax is 4095, an odd number.
	data := []struct {
		srcHz, desiredHz uint64
		x, y             int
		actualX, actualY int
		actualHz         uint64
	}{
		{
			// 93795/46886 = 2.0004 (error: 0.025%)
			192000000, 192000000 / clockDiviMax,
			clockDiviMax, dmaWaitcyclesMax + 1,
			89, 23, 93795,
		},
		{
			// 12800000/6193548 = 2.0666x (error: 3%)
			192000000, 192000000 / (dmaWaitcyclesMax + 1),
			clockDiviMax, dmaWaitcyclesMax + 1,
			32, 1, 6000000,
		},
		{
			// 2930/1465 = 2x
			192000000, 192000000 / clockDiviMax / (dmaWaitcyclesMax + 1),
			clockDiviMax, dmaWaitcyclesMax + 1,
			2427, 27, 2930,
		},
		{
			// Lowest clean clock.
			192000000, 1500,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 32, 1500,
		},
		{
			// Large amount of oversampling but 1500 samples/s is not a big deal
			// anyway.
			192000000, 100,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 32, 1500,
		},
		{
			192000000, 10,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 32, 1500,
		},
		{
			192000000, 1,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 32, 1500,
		},
		{
			192000000, 2,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 32, 1500,
		},
		{
			// 1465.2014/7 = 209.31x (error: 0.15%)
			192000000, 7,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4095, 32, 1465, // 1465.2014
		},
		{
			// Oversample by 2x
			192000000, 1000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 24, 2000,
		},
		{
			192000000, 2000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 24, 2000,
		},
		{
			192000000, 2500,
			clockDiviMax, dmaWaitcyclesMax + 1,
			3840, 20, 2500,
		},
		{
			192000000, 3000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			4000, 16, 3000,
		},
		{
			192000000, 10000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			3840, 5, 10000,
		},
		{
			192000000, 100000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			1920, 1, 100000,
		},
		{
			192000000, 120000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			1600, 1, 120000,
		},
		{
			192000000, 125000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			1536, 1, 125000,
		},
		{
			500000000, 1000000,
			clockDiviMax, dmaWaitcyclesMax + 1,
			500, 1, 1000000,
		},
	}
	for i, line := range data {
		line := line
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			t.Parallel()
			actualX, actualY, actualHz, _ := findDivisor(line.srcHz, line.desiredHz, line.x, line.y)
			if line.actualX != actualX || line.actualY != actualY || line.actualHz != actualHz {
				t.Fatalf("findDivisor(%d, %d, %d, %d) = %d, %d, %d  expected %d, %d, %d",
					line.srcHz, line.desiredHz, line.x, line.y, actualX, actualY, actualHz, line.actualX, line.actualY, line.actualHz)
			}
		})
	}
}

func BenchmarkFindDivisor_Exact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		findDivisor(192000000, 120000, clockDiviMax, dmaWaitcyclesMax)
	}
}

func BenchmarkFindDivisor_Fuzzy(b *testing.B) {
	// TODO(maruel): It is really too slow.
	for i := 0; i < b.N; i++ {
		findDivisor(192000000, 3, clockDiviMax, dmaWaitcyclesMax)
	}
}
