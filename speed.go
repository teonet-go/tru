// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU calculate speed per second module

package tru

import "time"

const sumArrayLen = 10

type speed struct {
	speed      int
	current    int
	currentSum int
	sumArray   [sumArrayLen]int
	timer      *time.Timer
}

// newSpeed create new packet speed calculator
func (s *speed) init() {
	s.process()
}

// destroy packet speed calculator
func (s *speed) destroy() {
	if s.timer != nil {
		s.timer.Stop()
	}
}

// get current speed
func (s *speed) get() int {
	return s.speed
}

// add packet to speed calculator
func (s *speed) add(count ...bool) {
	now := time.Now()
	unixNano := now.UnixNano()
	umillisec := unixNano / 1000000
	i := int((umillisec / 100) % 10)
	if i != s.current {
		s.sumArray[s.current] = s.currentSum
		speed := 0
		for _, v := range s.sumArray {
			speed += v
		}
		s.speed = speed
		s.currentSum = 0
		s.current = i
	}

	if len(count) == 0 {
		s.currentSum++
	}
}

// process executes 10 times per second to switch spped array when packets not
// received
func (s *speed) process() {
	s.timer = time.AfterFunc(100*time.Millisecond, func() {
		s.add(false)
		s.process()
	})
}
