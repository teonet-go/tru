// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"log"
	"net"
)

type Channel struct {
	addr       net.Addr   // Peer address
	mode       bool       // Server mode if true
	id         uint16     // Next send ID
	expectedID uint16     // Next expected ID
	reader     ReaderFunc // Channels reader
	tru        *Tru       // Pointer to tru
}

// const MaxUint16 = ^uint16(0)

// Addr return channel address
func (ch *Channel) Addr() net.Addr {
	return ch.addr
}

// WriteTo writes a packet with data to channel
func (ch *Channel) WriteTo(data []byte) (err error) {
	ch.tru.senderCh <- senderChData{ch, data}
	return
}

// newID create new channels packet id
func (tru *Tru) newID(ch *Channel) (id int) {
	tru.m.Lock()
	defer tru.m.Unlock()

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
		ch.mode = serverMode[0]
	}
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
