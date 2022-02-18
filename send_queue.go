// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Send Queue module

package tru

import (
	"container/list"
	"fmt"
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
	maxRetransmitAttempts = 100
)

// init send queue
func (s *sendQueue) init(ch *Channel) {
	s.index = make(map[uint16]*list.Element)
	s.retransmit(ch)
}

// destroy send queue
func (s *sendQueue) destroy() {
	s.Lock()
	defer s.Unlock()
	s.retransmitTimer.Stop()
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

	e, pac, ok := s.get(id, true) // unsafe get (does not lock)
	if ok {
		s.queue.Remove(e)
		delete(s.index, uint16(id))
		log.Println("delete from send queue", pac.ID())
	}
	return
}

// get packet from send queue. Does not lock/unlock if seconf parameter true
func (s *sendQueue) get(id int, unsafe ...bool) (e *list.Element, pac *Packet, ok bool) {
	if len(unsafe) == 0 || !unsafe[0] {
		s.RLock()
		defer s.RUnlock()
	}

	e, ok = s.index[uint16(id)]
	if ok {
		pac = e.Value.(*Packet)
	}
	return
}

// getFirst return first queu element or nil if queue is empty
func (s *sendQueue) getFirst() (pac *Packet) {
	s.RLock()
	defer s.RUnlock()

	e := s.queue.Front()
	if e == nil {
		return
	}

	pac = e.Value.(*Packet)
	return
}

// getRetransmitAttempts return retransmit attmenpts of first queu element or
// 0 if queue is empty
func (s *sendQueue) getRetransmitAttempts() (rta int) {
	pac := s.getFirst()
	if pac == nil {
		return
	}

	return pac.getRetransmitAttempts()
}

// len return send queue len
func (s *sendQueue) len() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.index)
}

// retransmit packets from send queue
func (s *sendQueue) retransmit(ch *Channel) {
	s.Lock()
	defer s.Unlock()

	s.retransmitTimer = time.AfterFunc( /* minRTT */ 100*time.Millisecond, func() {

		s.RLock()

		// Retransmit packets from send queue while retransmit
		// time before now
		for e := s.queue.Front(); e != nil; e = e.Next() {

			// Check retransmit time
			pac := e.Value.(*Packet)
			if !pac.retransmitTime.Before(time.Now()) {
				break
			}

			// Resend packet and set new retransmitTime
			rta := pac.retransmitAttempts + 1
			pac.setRetransmitAttempts(rta)
			if rta > maxRetransmitAttempts {
				s.RUnlock()
				ch.destroy(fmt.Sprint("channel max retransmit, destroy ", ch.addr.String()))
				return
			}
			ch.setRetransmitTime(pac)

			// Send to write channel
			ch.writeToSender(pac)
			ch.stat.setRetransmit()

			// Does not retranmit another packets if this has more than 1
			// retransmit attempts
			// if pac.retransmitAttempts > 1 {
			// 	break
			// }
		}

		s.RUnlock()
		s.retransmit(ch)
	})
}
