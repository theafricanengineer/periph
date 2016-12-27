// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package sk6812rgbw is a driver for SK6812RGBW LEDs.
//
// This driver is specifically for RGB+W LEDs. This mean each LED has 4
// individual channels. The white LED is usually either quite cold (~6500K) or
// quite warm (~3000K).
//
// For RGB SK6812 without a dedicated white channel, use the ws281x compatible
// driver.
//
// Datasheet
//
// https://github.com/cpldcpu/light_ws2812/tree/master/Datasheets
package sk6812rgbw
