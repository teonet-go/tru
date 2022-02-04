// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Connect module

package tru

import (
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
)

type connectData struct {
	uuid string
	wch  chan *connectData
	ch   *Channel
}

// Connect to new channel by address
func (tru *Tru) Connect(addr string, reader ...ReaderFunc) (ch *Channel, err error) {

	// Create uuid and connect packet
	uuid := uuid.New().String()
	pac, err := tru.newPacket().SetStatus(statusConnect).SetData([]byte(uuid)).MarshalBinary()
	if err != nil {
		return
	}

	// Create wait connection channel and save connection data to map
	wch := tru.addConnect(uuid)
	defer tru.deleteConnect(uuid)
	defer close(wch)

	// Send connect message
	tru.writeTo(pac, addr)
	if err != nil {
		return
	}

	// Wait answer to connect message or timeout
	ch, err = tru.waitConnect(wch)
	if err != nil {
		return
	}

	// Add reader to channel
	if len(reader) > 0 {
		ch.reader = reader[0]
	}
	return
}

// waitConnect channel connected or timeout
func (tru *Tru) waitConnect(wch chan *connectData) (ch *Channel, err error) {
	select {
	case cd := <-wch:
		ch = cd.ch
		return
	case <-time.After(5 * time.Second):
		err = errors.New("can't connect to peer during timeout")
		return
	}
}

// addConnect add connection data to connections map
func (tru *Tru) addConnect(uuid string) (wch chan *connectData) {
	tru.m.Lock()
	defer tru.m.Unlock()
	wch = make(chan *connectData)
	tru.connects[uuid] = &connectData{uuid, wch, nil}
	return
}

// deleteConnect delete connection data from connections map
func (tru *Tru) deleteConnect(uuid string) {
	tru.m.Lock()
	defer tru.m.Unlock()
	delete(tru.connects, uuid)
}

// getConnect get connection data from connections map
func (tru *Tru) getConnect(uuid string) (cd *connectData, ok bool) {
	tru.m.RLock()
	defer tru.m.RUnlock()
	cd, ok = tru.connects[uuid]
	return
}

// serveConnect process connect packets
func (tru *Tru) serveConnect(addr net.Addr, pac *Packet) (err error) {
	switch pac.Status() {

	case statusConnect:
		var ch *Channel
		ch, err = tru.newChannel(addr, true)
		if err != nil {
			return
		}
		var data []byte
		data, err = tru.newPacket().SetStatus(statusConnectAnswer).SetData(pac.Data()).MarshalBinary()
		if err != nil {
			return
		}
		err = tru.writeTo(data, ch.addr)

	case statusConnectAnswer:
		cd, ok := tru.getConnect(string(pac.Data()))
		if !ok {
			err = errors.New("wrong connect answer packet")
			return
		}
		cd.ch, err = tru.newChannel(addr)
		if err != nil {
			return
		}
		cd.wch <- cd

	default:
		err = errors.New("wrong packet status")
	}
	return
}
