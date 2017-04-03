// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package ssd1306smoketest is leveraged by periph-smoketest to verify that two
// SSD1306, one over I²C, one over SPI, can display the same output.
package ssd1306smoketest

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"log"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/i2c/i2ctest"
	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/conn/spi/spitest"
	"periph.io/x/periph/devices/ssd1306"
	"periph.io/x/periph/devices/ssd1306/image1bit"
)

// SmokeTest is imported by periph-smoketest.
type SmokeTest struct {
}

func (s *SmokeTest) String() string {
	return s.Name()
}

// Name implements the SmokeTest interface.
func (s *SmokeTest) Name() string {
	return "ssd1306"
}

// Description implements the SmokeTest interface.
func (s *SmokeTest) Description() string {
	return "Tests SSD1306 over I²C and SPI by displaying multiple patterns that exercises all code paths"
}

// Run implements the SmokeTest interface.
func (s *SmokeTest) Run(args []string) (err error) {
	f := flag.NewFlagSet("buses", flag.ExitOnError)
	i2cName := f.String("i2c", "", "I²C bus to use")
	spiName := f.String("spi", "", "SPI bus to use")
	dcName := f.String("dc", "", "DC pin to use in 4-wire SPI mode")

	w := f.Int("w", 128, "Display width")
	h := f.Int("h", 64, "Display height")
	rotated := f.Bool("rotated", false, "Rotate the displays by 180°")

	record := f.Bool("record", false, "record operation (for playback unit testing)")
	f.Parse(args)

	i2cBus, err2 := i2creg.Open(*i2cName)
	if err2 != nil {
		return err2
	}
	defer func() {
		if err2 := i2cBus.Close(); err == nil {
			err = err2
		}
	}()

	spiBus, err2 := spireg.Open(*spiName)
	if err2 != nil {
		return err2
	}
	defer func() {
		if err2 := spiBus.Close(); err == nil {
			err = err2
		}
	}()

	var dc gpio.PinOut
	if len(*dcName) != 0 {
		dc = gpioreg.ByName(*dcName)
	}
	if !*record {
		return s.run(i2cBus, spiBus, dc, *w, *h, *rotated)
	}

	i2cRecorder := i2ctest.Record{Bus: i2cBus}
	spiRecorder := spitest.Record{Conn: spiBus}
	err = s.run(&i2cRecorder, &spiRecorder, dc, *w, *h, *rotated)
	if len(i2cRecorder.Ops) != 0 {
		fmt.Printf("I²C recorder Addr: 0x%02X\n", i2cRecorder.Ops[0].Addr)
	} else {
		fmt.Print("I²C recorder\n")
	}
	for _, op := range i2cRecorder.Ops {
		fmt.Print("  Write: ")
		for i, b := range op.Write {
			if i != 0 {
				fmt.Print(", ")
			}
			fmt.Printf("0x%02X", b)
		}
		fmt.Print("\n   Read: ")
		for i, b := range op.Read {
			if i != 0 {
				fmt.Print(", ")
			}
			fmt.Printf("0x%02X", b)
		}
		fmt.Print("\n")
	}
	fmt.Print("\nSPI recorder\n")
	for _, op := range spiRecorder.Ops {
		fmt.Print("  Write: ")
		if len(op.Read) != 0 {
			// Read data.
			fmt.Printf("0x%02X\n   Read: ", op.Write[0])
			// first byte is dummy.
			for i, b := range op.Read[1:] {
				if i != 0 {
					fmt.Print(", ")
				}
				fmt.Printf("0x%02X", b)
			}
		} else {
			// Write-only command.
			for i, b := range op.Write {
				if i != 0 {
					fmt.Print(", ")
				}
				fmt.Printf("0x%02X", b)
			}
			fmt.Print("\n   Read: ")
		}
		fmt.Print("\n")
	}
	return err
}

func (s *SmokeTest) run(i2cBus i2c.Bus, spiBus spi.ConnCloser, dc gpio.PinOut, w, h int, rotated bool) (err error) {
	i2cDev, err2 := ssd1306.NewI2C(i2cBus, w, h, rotated)
	if err2 != nil {
		return err2
	}
	spiDev, err2 := ssd1306.NewSPI(spiBus, dc, w, h, rotated)
	if err2 != nil {
		return err2
	}
	devices := []*ssd1306.Dev{i2cDev, spiDev}

	log.Printf("%s: Image Bunny (slow path: NRGBA)", s)
	imgBunny, err := gif.Decode(bytes.NewReader(bunny))
	if err != nil {
		return err
	}
	for _, s := range devices {
		s.Draw(s.Bounds(), imgBunny, image.Point{})
	}
	time.Sleep(1 * time.Second)

	log.Printf("%s: Image Bunny (faster path: image1bit)", s)
	imgBunny1bit := image1bit.New(imgBunny.Bounds())
	draw.Src.Draw(imgBunny1bit, imgBunny.Bounds(), imgBunny, image.Point{})
	for _, s := range devices {
		s.Draw(s.Bounds(), imgBunny1bit, image.Point{})
	}
	time.Sleep(1 * time.Second)

	log.Printf("%s: Image Bunny (fastest path: image1bit exact frame size)", s)
	imgBunny1bitLarge := image1bit.New(i2cDev.Bounds())
	draw.Src.Draw(imgBunny1bitLarge, imgBunny1bit.Bounds(), imgBunny1bit, image.Point{})
	for _, s := range devices {
		s.Draw(s.Bounds(), imgBunny1bitLarge, image.Point{})
	}
	time.Sleep(1 * time.Second)

	log.Printf("%s: Scroll left", s)
	for _, s := range devices {
		if err := s.Scroll(ssd1306.Left, ssd1306.FrameRate2, 0, -1); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	log.Printf("%s: Scroll right", s)
	for _, s := range devices {
		if err := s.Scroll(ssd1306.Right, ssd1306.FrameRate2, 0, -1); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	log.Printf("%s: Scroll up left", s)
	for _, s := range devices {
		if err := s.Scroll(ssd1306.UpLeft, ssd1306.FrameRate2, 0, -1); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	log.Printf("%s: Scroll up right", s)
	for _, s := range devices {
		if err := s.Scroll(ssd1306.UpRight, ssd1306.FrameRate2, 0, -1); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	log.Printf("%s: Stop scroll", s)
	for _, s := range devices {
		if err := s.StopScroll(); err != nil {
			return err
		}
	}

	log.Printf("%s: contrast 0", s)
	for _, s := range devices {
		if err := s.SetContrast(0); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	log.Printf("%s: contrast 0xFF", s)
	for _, s := range devices {
		if err := s.SetContrast(0xFF); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)

	// Create synthetic images using a raw array. Each byte corresponds to 8
	// vertical pixels, and then the array scans horizontally and down.
	log.Printf("%s: broad stripes", s)
	var img [128 * 64 / 8]byte
	for y := 0; y < 8; y++ {
		// Horizontal stripes.
		for x := 0; x < 64; x++ {
			img[x+128*y] = byte((y & 1) * 0xff)
		}
		// Vertical stripes.
		for x := 64; x < 128; x++ {
			img[x+128*y] = byte(((x / 8) & 1) * 0xff)
		}
	}
	for _, s := range devices {
		if _, err := s.Write(img[:]); err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond)

	// Display off and back on.
	log.Printf("%s: Off & on", s)
	for _, s := range devices {
		if err := s.Enable(false); err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond)
	for _, s := range devices {
		if err := s.Enable(true); err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond)

	log.Printf("%s: Invert the display and restore", s)
	for _, s := range devices {
		if err := s.Invert(true); err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond)

	for _, s := range devices {
		if err := s.Invert(false); err != nil {
			return err
		}
	}
	time.Sleep(500 * time.Millisecond)

	log.Printf("%s: Change the contrast", s)
	for c := 0; c < 256; c++ {
		for _, s := range devices {
			if err := s.SetContrast(byte(c)); err != nil {
				return err
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, s := range devices {
		s.SetContrast(0xff)
	}

	log.Printf("%s: Fill display with binary 0..255 pattern", s)
	for i := 0; i < len(img); i++ {
		img[i] = byte(i)
	}
	for _, s := range devices {
		if _, err := s.Write(img[:]); err != nil {
			return err
		}
	}

	return nil
}
