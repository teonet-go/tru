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
	done chan bool
}

// NewSpeed creates new Speed object and start calculate packets speed.
// It should be closed after usage with Close() method.
func NewSpeed() *Speed {
	s := &Speed{
		packets: make([]int, curent+1),
		RWMutex: new(sync.RWMutex),
		done:    make(chan bool),
	}
	go s.process()
	return s
}

// Close closes speed object.
func (s *Speed) Close() {
	s.done <- true
}

// Speed returns packets per second.
func (s *Speed) Speed() int {
	s.RLock()
	defer s.RUnlock()
	return s.speed
}

// Add adds one packet or just tick to switch part of speed calculation.
func (s *Speed) Add(tick ...any) {
	s.Lock()
	defer s.Unlock()

	part := time.Now().Nanosecond() / 100000000
	if part != s.current {
		s.current = part
		s.packets = append(s.packets[1:], 0)
		s.speed = s.calculate()
	}

	if len(tick) == 0 {
		s.packets[curent]++
	}
}

// calculate calculate and returns packets per second.
func (s *Speed) calculate() (speed int) {

	for i := 0; i < curent; i++ {
		speed += s.packets[i]
	}

	return
}

// process emulate adding packets to show real speed when packets are not sent.
func (s *Speed) process() {
	tick := time.NewTicker(time.Second / curent)
	for {
		select {
		// Get done signal to stop this process
		case <-s.done:
			tick.Stop()
			return
		// Get tick signal to emulate adding packets in this period
		case <-tick.C:
			s.Add(nil)
		}
	}
}
