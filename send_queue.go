// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"container/list"
	"log"
	"sync"
	"time"
)

type sendQueue struct {
	queue list.List                // Send queue list
	index map[uint16]*list.Element // Send queue index
	m     sync.RWMutex             // Send queue mutex
}

const (
	minRTT = 30 * time.Millisecond
	maxRTT = 3000 * time.Millisecond
	startRTT = 200 * time.Millisecond
)

// init send queue
func (s *sendQueue) init() {
	s.index = make(map[uint16]*list.Element)
}

// add packet to send queue
func (s *sendQueue) add(pac *Packet) {
	s.m.Lock()
	defer s.m.Unlock()

	id := uint16(pac.ID())
	s.index[id] = s.queue.PushBack(pac)
	log.Println("add to send queue", pac.ID())
}

// delete packet from send queue
func (s *sendQueue) delete(id int) (pac *Packet, ok bool) {
	s.m.Lock()
	defer s.m.Unlock()

	e, pac, ok := s.get(id)
	if ok {
		s.queue.Remove(e)
		delete(s.index, uint16(id))
		log.Println("delete from send queue", pac.ID())
	}
	return
}

// get packet in sendQueue
func (s *sendQueue) get(id int) (e *list.Element, pac *Packet, ok bool) {
	e, ok = s.index[uint16(id)]
	if ok {
		pac = e.Value.(*Packet)
	}
	return
}
