// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Send Queue module

package tru

import (
	"container/list"
	"log"
	"sync"
	"time"
)

type sendQueue struct {
	queue           list.List                // Send queue list
	index           map[uint16]*list.Element // Send queue index
	sync.RWMutex                             // Send queue mutex
	retransmitTimer *time.Timer              // Send queue retransmit Timer
}

const (
	minRTT                = 30 * time.Millisecond
	maxRTT                = 3000 * time.Millisecond
	startRTT              = 200 * time.Millisecond
	maxRetransmitAttempts = 5
)

// init send queue
func (s *sendQueue) init(ch *Channel) {
	s.index = make(map[uint16]*list.Element)
	s.retransmit(ch)
}

// add packet to send queue
func (s *sendQueue) add(pac *Packet) {
	s.Lock()
	defer s.Unlock()

	id := uint16(pac.ID())
	s.index[id] = s.queue.PushBack(pac)
	log.Println("add to send queue", pac.ID())
}

// delete packet from send queue
func (s *sendQueue) delete(id int) (pac *Packet, ok bool) {
	s.Lock()
	defer s.Unlock()

	e, pac, ok := s.get(id, false)
	if ok {
		s.queue.Remove(e)
		delete(s.index, uint16(id))
		log.Println("delete from send queue", pac.ID())
	}
	return
}

// get packet from send queue
func (s *sendQueue) get(id int, lock ...bool) (e *list.Element, pac *Packet, ok bool) {
	if len(lock) == 0 || lock[0] {
		s.RLock()
		defer s.RUnlock()
	}

	e, ok = s.index[uint16(id)]
	if ok {
		pac = e.Value.(*Packet)
	}
	return
}

// len return send queue len
func (s *sendQueue) len() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.index)
}

// retransmit packets from send queue
func (s *sendQueue) retransmit(ch *Channel) {
	s.retransmitTimer = time.AfterFunc(minRTT, func() {

		s.RLock()
		defer s.RUnlock()

		// Retransmit packets from send queue while retransmit
		// time before now
		for e := s.queue.Front(); e != nil; e = e.Next() {
			// Check retransmit time
			pac := e.Value.(*Packet)
			if !pac.retransmitTime.Before(time.Now()) {
				break
			}
			// Resend packet and set new retransmitTime
			pac.retransmitAttempts++
			if pac.retransmitAttempts > maxRetransmitAttempts {
				ch.destroy()
				return
			}
			ch.setRetransmitTime(pac)
			data, _ := pac.MarshalBinary()
			ch.tru.writeTo(data, ch.addr)
			ch.stat.retransmit++
			log.Println("!!!  retransmit id:", pac.ID(), "sq:", len(s.index))
		}

		s.retransmit(ch)
	})
}
