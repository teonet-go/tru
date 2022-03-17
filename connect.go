// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Connect module

package tru

import (
	"bytes"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

const waitConnectionTimeout = 5 * time.Second
const (
	ClientConnectTimeout = waitConnectionTimeout
	ServerConnectTimeout = waitConnectionTimeout
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

type connectPacketData struct {
	uuid []byte // Connection UUID
	data []byte // Packet data
}

// MarshalBinary marshal connection data
func (c *connectPacketData) MarshalBinary() (out []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	binary.Write(buf, le, uint8(len(c.uuid)))
	binary.Write(buf, le, c.uuid)
	binary.Write(buf, le, c.data)

	out = buf.Bytes()
	return
}

// UnmarshalBinary unmarshal connection data
func (c *connectPacketData) UnmarshalBinary(data []byte) (err error) {

	buf := bytes.NewReader(data)
	le := binary.LittleEndian

	var l uint8
	err = binary.Read(buf, le, &l)
	if err != nil {
		return
	}

	uuid := make([]byte, l)
	err = binary.Read(buf, le, &uuid)
	if err != nil {
		return
	}
	c.uuid = uuid

	if buflen := buf.Len(); buflen > 0 {
		c.data = make([]byte, buflen)
		err = binary.Read(buf, le, &c.data)
		return
	}
	c.data = nil
	return
}

// Connect to tru channel (remote peer) by address
func (tru *Tru) Connect(addr string, reader ...ReaderFunc) (ch *Channel, err error) {

	// Generate RSA key
	c, err := tru.newCrypt()
	if err != nil {
		return
	}
	pub, err := c.publicKeyToBytes(&c.privateKey.PublicKey)
	if err != nil {
		return
	}

	// Create uuid and connect packet
	uuid := uuid.New().String()
	cp := connectPacketData{[]byte(uuid), pub}
	data, err := cp.MarshalBinary()
	if err != nil {
		return
	}
	pac, err := tru.newPacket().SetStatus(statusConnect).SetData(data).MarshalBinary()
	if err != nil {
		return
	}

	// Create wait connection channel and save connection data to map
	wch := tru.connect.add(uuid)
	defer tru.connect.delete(uuid)
	defer close(wch)

	// Send connect message
	_, err = tru.WriteTo(pac, addr)
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
	case <-time.After(waitConnectionTimeout):
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

	// Got by server. Client send to server statusConnect packet with client
	// public key
	case statusConnect:

		// Unmarshal received data
		cp := connectPacketData{}
		err = cp.UnmarshalBinary(pac.Data())
		if err != nil {
			return
		}

		// Create new tru channel
		var ch *Channel
		ch, err = tru.newChannel(addr, true)
		if err != nil {
			return
		}

		// Get public key
		var pub []byte
		pub, err = ch.publicKeyToBytes(&ch.privateKey.PublicKey)
		if err != nil {
			return
		}

		// Encrypt public key with received clients public key
		var pubcli *rsa.PublicKey
		pubcli, err = ch.bytesToPublicKey(cp.data)
		if err != nil {
			return
		}
		cp.data, err = ch.encrypt(pubcli, pub)
		if err != nil {
			return
		}
		var data []byte
		data, err = cp.MarshalBinary()

		// Create packet and send it to tru channel
		pac = tru.newPacket().SetStatus(statusConnectServerAnswer).SetData(data)
		ch.writeToSender(pac)

	// Got by client. Server answer to client with statusConnectServerAnswer
	// packet with server public key
	case statusConnectServerAnswer:

		// Unmarshal received data
		cp := connectPacketData{}
		err = cp.UnmarshalBinary(pac.Data())
		if err != nil {
			return
		}

		// Get connection data from connection map and create new tru channel
		cd, ok := c.get(string(cp.uuid))
		if !ok {
			err = errors.New("wrong connect server answer packet")
			return
		}
		cd.ch, err = tru.newChannel(addr)
		if err != nil {
			return
		}

		// Got servers public key from packet
		var data []byte
		data, err = cd.ch.decrypt(cp.data)
		if err != nil {
			return
		}
		var pub *rsa.PublicKey
		pub, err = cd.ch.crypt.bytesToPublicKey(data)
		if err != nil {
			return
		}

		// Make session key
		key := cd.ch.makeSesionKey()
		cp.data, err = cd.ch.encrypt(pub, key)
		if err != nil {
			return
		}

		// Create output connect packet data
		data, err = cp.MarshalBinary()
		if err != nil {
			return
		}

		// Create packet and send it to tru channel
		pac = tru.newPacket().SetStatus(statusConnectClientAnswer).SetData(data)
		cd.ch.writeToSender(pac)
		cd.ch.setSesionKey(key)

	// Got by server. Client answer to server with statusConnectClientAnswer packet with
	// current session key
	case statusConnectClientAnswer:

		// Unmarshal received data
		cp := connectPacketData{}
		err = cp.UnmarshalBinary(pac.Data())
		if err != nil {
			return
		}

		// Get channel
		ch, ok := tru.getChannel(addr.String())
		if !ok {
			err = errors.New("connected channel does not exists")
			return
		}

		// Decrypt and set session key
		var key []byte
		key, err = ch.decrypt(cp.data)
		if err != nil {
			return
		}
		ch.setSesionKey(key)

		// Create output connect packet data
		var data []byte
		cp.data = nil
		data, err = cp.MarshalBinary()
		if err != nil {
			return
		}

		// Create packet and send it to tru channel
		pac = tru.newPacket().SetStatus(statusConnectDone).SetData(data)
		ch.writeToSender(pac)

	// Got by client. Server answer to client with statusConnectDone packet
	case statusConnectDone:

		// Unmarshal received data
		cp := connectPacketData{}
		err = cp.UnmarshalBinary(pac.Data())
		if err != nil {
			return
		}

		// Get connection data from connection map and get tru channel
		cd, ok := c.get(string(cp.uuid))
		if !ok {
			err = errors.New("wrong connect done packet")
			return
		}

		// Send connectData to client connect wait channel
		cd.wch <- cd

	default:
		err = errors.New("wrong packet status")
	}

	return
}
