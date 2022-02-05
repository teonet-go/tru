// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"errors"
	"log"
	"net"
)

type Channel struct {
	addr       net.Addr   // Peer address
	serverMode bool       // Server mode if true
	id         uint16     // Next send ID
	expectedID uint16     // Next expected ID
	reader     ReaderFunc // Channels reader
	stat       statistic  // Statictic struct and receiver
	tru        *Tru       // Pointer to tru
}

// const MaxUint16 = ^uint16(0)

// Addr return channel address
func (ch *Channel) Addr() net.Addr {
	return ch.addr
}

// WriteTo writes a packet with data to channel
func (ch *Channel) WriteTo(data []byte) (err error) {
	return ch.writeTo(data, statusData)
}

// writeTo writes a packet with status and data to channel
func (ch *Channel) writeTo(data []byte, status int) (err error) {
	if ch.stat.destroyed {
		err = errors.New("channel destroyed")
		return
	}

	ch.tru.senderCh <- senderChData{ch, data, status}
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

// newID create new channels packet id
func (ch *Channel) newID() (id int) {
	ch.tru.m.Lock()
	defer ch.tru.m.Unlock()

	id = int(ch.id)
	ch.id++

	return
}

// NewChannel create new tru channel by address
func (tru *Tru) newChannel(addr net.Addr, serverMode ...bool) (ch *Channel, err error) {
	tru.m.Lock()
	defer tru.m.Unlock()

	log.Println("new channel", addr.String())

	ch = &Channel{addr: addr, tru: tru}
	if len(serverMode) > 0 {
		ch.serverMode = serverMode[0]
	}
	ch.stat.setLastActivity()
	ch.stat.checkActivity(
		// Inactive
		func() {
			log.Println("channel inactive", ch.addr.String())
			tru.destroyChannel(ch)
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

// destroyChannel destroy channel
func (tru *Tru) destroyChannel(ch *Channel) {
	tru.m.Lock()
	defer tru.m.Unlock()

	if ch == nil {
		return
	}
	log.Println("channel destroy", ch.addr.String())

	ch.stat.checkActivityTimer.Stop()
	ch.stat.destroyed = true

	delete(tru.cannels, ch.addr.String())
}
