// Copyright 2023 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru Calculate Speed packets/sec module

package tru

import (
	"sync"
	"time"
)

const curent = 10

// Speed struct for calculate speed packets/sec.
type Speed struct {
	packets []int
	current int
	speed   int
	*sync.RWMutex
}

// NewSpeed creates new Speed object.
func NewSpeed() *Speed {
	return &Speed{packets: make([]int, curent+1), RWMutex: new(sync.RWMutex)}
}

// Speed returns packets per second.
func (s *Speed) Speed() int {
	s.RLock()
	defer s.RUnlock()
	return s.speed
}

// Add adds one packet.
func (s *Speed) Add() {
	s.Lock()
	defer s.Unlock()

	part := time.Now().Nanosecond() / 100000000
	if part != s.current {
		s.current = part
		s.packets = append(s.packets[1:], 0)
		s.speed = s.calculate()
	}

	s.packets[curent]++
}

// calculate calculate and returns packets per second.
func (s *Speed) calculate() (speed int) {

	for i := 0; i < curent; i++ {
		speed += s.packets[i]
	}

	return
}
