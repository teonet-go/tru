// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Packet module

package tru

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

// Packet status
const (
	statusConnect = iota
	statusConnectServerAnswer
	statusConnectClientAnswer
	statusConnectDone
	statusData
	statusAck
	statusPing
	statusPong
	statusDisconnect
	statusPunch
	statusSplit    = 0x80
	statusDataNext = statusData + statusSplit
)

const (
	maxUdpDataLength = 65535 - 8 // Max packet 65535 - 8 byte header
	cryptAesLength   = 28        // AES crypt delete from max len packet data
)

const packetIDLimit = 0x100000          // Number of packets id (max number + 1)
const DeliveryTimeout = 5 * time.Second // Default delivery function timeout

// Packet struct
type Packet struct {
	id                 uint32             // Packet ID
	status             uint8              // Packet Type
	data               []byte             // Packet Data
	time               time.Time          // Packet creating time
	retransmitTime     time.Time          // Packet retransmit time
	retransmitAttempts int                // Packet retransmit attempts
	delivery           PacketDeliveryFunc // Packet delivery callback function
	deliveryTimeout    time.Duration      // Packet delivery callback timeout
	deliveryTimer      time.Timer         // Packet delivery timeout timer
	sync.RWMutex
}

// PacketDeliveryFunc packet delivery callback function calls when packet
// delivered to remote peer
type PacketDeliveryFunc func(pac *Packet, err error)

// newPacket create new empty packet
func (tru *Tru) newPacket() *Packet {
	return &Packet{time: time.Now()}
}

// MarshalBinary marshal packet
//
//	Bynary packet structure:
//	+--------------------+------+
//	| ID & STATUS uint32 | DATA |
//	+--------------------+------+
//	ID & STATUS uint32: STATUS 1 byte | ID 3 byte
func (p *Packet) MarshalBinary() (out []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	// Pack status & id
	statid := p.packStatID()

	// Write status&id and data to buffer
	binary.Write(buf, le, statid)
	binary.Write(buf, le, p.data)

	out = buf.Bytes()
	return
}

// UnmarshalBinary unmarshal packet
func (p *Packet) UnmarshalBinary(data []byte) (err error) {

	buf := bytes.NewReader(data)
	le := binary.LittleEndian

	// Read status&id from buffer and unpack it to p.status and p.id
	var statid uint32
	err = binary.Read(buf, le, &statid)
	if err != nil {
		return
	}
	p.unpackStatID(statid)

	// Read data from buffer
	if l := buf.Len(); l > 0 {
		p.data = make([]byte, l)
		err = binary.Read(buf, le, &p.data)
	}

	return
}

// packStatID pack status and id from packet
func (p *Packet) packStatID() uint32 {
	return p.id&(packetIDLimit-1) | uint32(p.status)<<24
}

// unpackStatID unpack status and id
func (p *Packet) unpackStatID(statid uint32) {
	p.id = statid & (packetIDLimit - 1)
	p.status = uint8(statid >> 24)
}

// HeaderLen get header length
func (p *Packet) HeaderLen() int {
	// Tru header:
	// id 		- 3 byte
	// status	- 1 byte
	return 4
}

// Len get packet length
func (p *Packet) MaxDataLen() int {
	return maxUdpDataLength - p.HeaderLen() - cryptAesLength
	// return 1024 // look like optimal packet data length
}

// Len get packet length
func (p *Packet) Len() int {
	return p.HeaderLen() + len(p.data)
}

// Data get packet data
func (p *Packet) Data() []byte {
	return p.data
}

// SetData set packet data
func (p *Packet) SetData(data []byte) *Packet {
	p.data = data
	return p
}

// ID get packet id
func (p *Packet) ID() int {
	return int(p.id)
}

// SetID set packet id
func (p *Packet) SetID(id int) *Packet {
	p.id = uint32(id)
	return p
}

// Status get packet status
func (p *Packet) Status() int {
	return int(p.status)
}

// SetStatus set packet status
func (p *Packet) SetStatus(status int) *Packet {
	p.status = uint8(status)
	return p
}

// Status get packet status
func (p *Packet) Delivery() PacketDeliveryFunc {
	return p.delivery
}

// SetDelivery set delivery function which calls when packet delivered to
// remote peer
func (p *Packet) SetDelivery(delivery PacketDeliveryFunc) *Packet {
	if delivery == nil {
		return p
	}
	log.Debugvvv.Println("set delivery func, id", p.ID())
	p.delivery = delivery
	p.deliveryTimer = *time.AfterFunc(p.deliveryTimeout, func() {
		// log.Error.Println("delivery timeout, id", p.ID())
		err := errors.New("delivery timeout")
		p.delivery(p, err)
	})
	return p
}

// SetDeliveryTimeout set delivery function timeout
func (p *Packet) SetDeliveryTimeout(timeout time.Duration) *Packet {
	log.Debugvvv.Println("set delivery timeout, id", p.ID())
	p.deliveryTimeout = timeout
	return p
}

// distance check received packet distance and return integer value
// lesse than zero than 'id < expectedID' or return integer value more than
// zero than 'id > tcd.expectedID'
func (p *Packet) distance(expectedID uint32, id uint32) int {
	if expectedID == id {
		return 0
	}

	// modSubU module of subtraction
	modSubU := func(arga, argb uint32, mod uint64) int {
		sub := (uint64(arga) % mod) + mod - (uint64(argb) % mod)
		return int(sub % mod)
	}

	diff := modSubU(id, expectedID, packetIDLimit)
	if diff < packetIDLimit/2 {
		return int(diff)
	}
	return int(diff - packetIDLimit)
}

// getRetransmitAttempts return retransmit attempts value. This function
// concurrent reads/writes saiflly in pair with setRetransmitAttempts()
func (p *Packet) getRetransmitAttempts() (rta int) {
	p.RLock()
	defer p.RUnlock()

	return p.retransmitAttempts
}

// setRetransmitAttempts set retransmit attempts value. This function
// concurrent reads/writes saiflly in pair with getRetransmitAttempts()
func (p *Packet) setRetransmitAttempts(rta int) {
	p.Lock()
	defer p.Unlock()

	p.retransmitAttempts = rta
}

// getRetransmitTime get retransmit time to packet
func (p *Packet) getRetransmitTime() (rtt time.Time) {
	p.RLock()
	defer p.RUnlock()

	return p.retransmitTime
}

// setRetransmitTime set retransmit time to packet
func (p *Packet) setRetransmitTime(rtt time.Duration) {
	p.Lock()
	defer p.Unlock()

	p.retransmitTime = time.Now().Add(rtt)
	p.time = time.Now()
}
