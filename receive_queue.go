// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Receive Queue module

package tru

import "sync"

type receiveQueue struct {
	ma           map[uint16]*Packet // Receive queue map
	sync.RWMutex                    // Receive queue mutex
}

// init receive queue
func (r *receiveQueue) init(ch *Channel) {
	r.ma = make(map[uint16]*Packet)
}

// add packet to receive queue
func (r *receiveQueue) add(pac *Packet) {
	r.Lock()
	defer r.Unlock()

	id := uint16(pac.ID())
	r.ma[id] = pac
}

// delete packet from receive queue
func (r *receiveQueue) delete(id int) (pac *Packet, ok bool) {
	r.Lock()
	defer r.Unlock()

	pac, ok = r.get(id, false)
	if ok {
		delete(r.ma, uint16(id))
	}
	return
}

// get packet from receive queue
func (r *receiveQueue) get(id int, lock ...bool) (pac *Packet, ok bool) {
	if len(lock) == 0 || lock[0] {
		r.RLock()
		defer r.RUnlock()
	}

	pac, ok = r.ma[uint16(id)]
	return
}

// len return receive queue len
func (r *receiveQueue) len() int {
	r.RLock()
	defer r.RUnlock()

	return len(r.ma)
}

// process find packets in received queue, send packets to user level and remove
// it from receive queue
func (r *receiveQueue) process(ch *Channel, send func(ch *Channel, pac *Packet)) (err error) {
	id := int(ch.expectedID)
	pac, ok := ch.recvQueue.get(id)
	if !ok {
		return
	}
	send(ch, pac)
	ch.newExpectedID()
	ch.stat.recv++
	ch.recvQueue.delete(id)

	return r.process(ch, send)
}
