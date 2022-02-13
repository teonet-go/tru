// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Connect module

package tru

import (
	"errors"
	"net"
	"sync"
	"time"

	// "github.com/google/uuid"
	uuid "github.com/nu7hatch/gouuid"
)

type connect struct {
	connects map[string]*connectData // Connections map
	m        sync.RWMutex            // Connections maps mutex
}

type connectData struct {
	uuid string
	wch  chan *connectData
	ch   *Channel
}

// Connect to new channel by address
func (tru *Tru) Connect(addr string, reader ...ReaderFunc) (ch *Channel, err error) {

	// Create uuid and connect packet
	// uuid := uuid.New().String()
	u, _ := uuid.NewV4()
	uuid := u.String()

	pac, err := tru.newPacket().SetStatus(statusConnect).SetData([]byte(uuid)).MarshalBinary()
	if err != nil {
		return
	}

	// Create wait connection channel and save connection data to map
	wch := tru.connect.add(uuid)
	defer tru.connect.delete(uuid)
	defer close(wch)

	// Send connect message
	tru.writeTo(pac, addr)
	if err != nil {
		return
	}

	// Wait answer to connect message or timeout
	ch, err = tru.connect.wait(wch)
	if err != nil {
		return
	}

	// Add reader to channel
	if len(reader) > 0 {
		ch.reader = reader[0]
	}
	return
}

// wait channel connected or timeout
func (c *connect) wait(wch chan *connectData) (ch *Channel, err error) {
	select {
	case cd := <-wch:
		ch = cd.ch
		return
	case <-time.After(5 * time.Second):
		err = errors.New("can't connect to peer during timeout")
		return
	}
}

// add add connection data to connections map
func (c *connect) add(uuid string) (wch chan *connectData) {
	c.m.Lock()
	defer c.m.Unlock()
	wch = make(chan *connectData)
	c.connects[uuid] = &connectData{uuid, wch, nil}
	return
}

// delete delete connection data from connections map
func (c *connect) delete(uuid string) {
	c.m.Lock()
	defer c.m.Unlock()
	delete(c.connects, uuid)
}

// get get connection data from connections map
func (c *connect) get(uuid string) (cd *connectData, ok bool) {
	c.m.RLock()
	defer c.m.RUnlock()
	cd, ok = c.connects[uuid]
	return
}

// serve process connect packets
func (c *connect) serve(tru *Tru, addr net.Addr, pac *Packet) (err error) {
	switch pac.Status() {

	case statusConnect:
		var ch *Channel
		ch, err = tru.newChannel(addr, true)
		if err != nil {
			return
		}
		pac = tru.newPacket().SetStatus(statusConnectAnswer).SetData(pac.Data())
		ch.writeToSender(pac)

	case statusConnectAnswer:
		cd, ok := c.get(string(pac.Data()))
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
