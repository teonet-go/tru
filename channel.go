// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"errors"
	"log"
	"net"
	"time"
)

type Channel struct {
	addr       net.Addr   // Peer address
	serverMode bool       // Server mode if true
	id         uint16     // Next send ID
	expectedID uint16     // Next expected ID
	reader     ReaderFunc // Channels reader
	stat       statistic  // Statictic struct and receiver
	sendQueue  sendQueue  // Send queue
	tru        *Tru       // Pointer to tru
}

// const MaxUint16 = ^uint16(0)

// NewChannel create new tru channel by address
func (tru *Tru) newChannel(addr net.Addr, serverMode ...bool) (ch *Channel, err error) {
	tru.m.Lock()
	defer tru.m.Unlock()

	log.Println("new channel", addr.String())

	ch = &Channel{addr: addr, tru: tru}
	if len(serverMode) > 0 {
		ch.serverMode = serverMode[0]
	}
	ch.sendQueue.init(ch)
	ch.stat.started = time.Now()
	ch.stat.setLastActivity()
	ch.stat.checkActivity(
		// Inactive
		func() {
			log.Println("channel inactive", ch.addr.String())
			ch.destroy()
		},
		// Keepalive
		func() {
			if ch.serverMode {
				return
			}
			log.Println("channel ping", ch.addr.String())
			ch.writeToPing()
		})

	tru.cannels[addr.String()] = ch
	return
}

// getChannel get tru channel by address
func (tru *Tru) getChannel(addr string) (ch *Channel, ok bool) {
	tru.m.RLock()
	defer tru.m.RUnlock()

	ch, ok = tru.cannels[addr]
	return
}

// Addr return channel address
func (ch *Channel) Addr() net.Addr {
	return ch.addr
}

// WriteTo writes a packet with data to channel
func (ch *Channel) WriteTo(data []byte) (err error) {
	return ch.writeTo(data, statusData)
}

// writeTo writes a packet with status and data to channel
func (ch *Channel) writeTo(data []byte, status int, ids ...int) (err error) {
	if ch.stat.destroyed {
		err = errors.New("channel destroyed")
		return
	}

	var id = 0
	if len(ids) > 0 {
		id = ids[0]
	}

	ch.tru.senderCh <- senderChData{ch, data, status, id}
	return
}

// writeToPing writes ping packet to channel
func (ch *Channel) writeToPing() (err error) {
	return ch.writeTo(nil, statusPing)
}

// writeToPong writes pong packet to channel
func (ch *Channel) writeToPong() (err error) {
	return ch.writeTo(nil, statusPong)
}

// writeToAck writes ack to packet to channel
func (ch *Channel) writeToAck(pac *Packet) (err error) {
	return ch.writeTo(nil, statusAck, pac.ID())
}

// newID create new channels packet id
func (ch *Channel) newID() (id int) {
	ch.tru.m.Lock()
	defer ch.tru.m.Unlock()

	id = int(ch.id)
	ch.id++

	return
}

// setTripTime calculate return and set trip time to statistic
func (ch *Channel) setTripTime(id int) (tt time.Duration, err error) {
	_, pac, ok := ch.sendQueue.get(id)
	if !ok {
		err = errors.New("packet not found")
		return
	}
	tt = time.Since(pac.time)
	ch.stat.tripTime = tt
	return
}

// setRetransmitTime set retransmit time to packet
func (ch *Channel) setRetransmitTime(pac *Packet) (tt time.Time, err error) {
	rtt := minRTT

	if ch.stat.tripTime == 0 {
		rtt = startRTT
	} else {
		rtt += ch.stat.tripTime
	}

	if pac.retransmitAttempts > 0 {
		rtt *= time.Duration(pac.retransmitAttempts + 1)
	}

	if rtt > maxRTT {
		rtt = maxRTT
	}

	tt = time.Now().Add(rtt)
	pac.retransmitTime = tt

	return
}

// destroy destroy channel
func (ch *Channel) destroy() {
	ch.tru.m.Lock()
	defer ch.tru.m.Unlock()

	if ch == nil {
		return
	}
	log.Println("channel destroy", ch.addr.String())

	ch.stat.checkActivityTimer.Stop()
	ch.sendQueue.retransmitTimer.Stop()
	ch.stat.destroyed = true

	delete(ch.tru.cannels, ch.addr.String())
}
