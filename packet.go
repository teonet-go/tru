// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Packet module

package tru

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"
)

type Packet struct {
	id                 uint16             // Packet ID
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
	statusSplit    = 0x80
	statusDataNext = statusData + statusSplit
)

const (
	maxUdpDataLength = 65527 // Max packet 65535 - 8 byte header
	cryptAesLength   = 28    // Ees crypt add to max len packet
)

const DeliveryTimeout = 5 * time.Second // Default delivery function timeout

// newPacket create new empty packet
func (tru *Tru) newPacket() *Packet {
	return &Packet{time: time.Now()}
}

// MarshalBinary marshal packet
func (p *Packet) MarshalBinary() (out []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	binary.Write(buf, le, p.id)
	binary.Write(buf, le, p.status)
	binary.Write(buf, le, p.data)

	out = buf.Bytes()
	return
}

// UnmarshalBinary unmarshal packet
func (p *Packet) UnmarshalBinary(data []byte) (err error) {

	buf := bytes.NewReader(data)
	le := binary.LittleEndian

	binary.Read(buf, le, &p.id)
	binary.Read(buf, le, &p.status)

	p.data = make([]byte, buf.Len())
	binary.Read(buf, le, &p.data)

	return
}

// HeaderLen get header length
func (p *Packet) HeaderLen() int {
	// Tru header:
	// id 		- 2 byte
	// status	- 1 byte
	return 3
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
	p.id = uint16(id)
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
	log.Println("set delivery func")
	p.delivery = delivery
	p.deliveryTimer = *time.AfterFunc(p.deliveryTimeout, func() {
		err := errors.New("delivery timeout")
		p.delivery(p, err)
	})
	return p
}

// SetDeliveryTimeout set delivery function timeout
func (p *Packet) SetDeliveryTimeout(timeout time.Duration) *Packet {
	log.Println("set delivery timeout")
	p.deliveryTimeout = timeout
	return p
}

// distance check received packet distance and return integer value
// lesse than zero than 'id < expectedID' or return integer value more than
// zero than 'id > tcd.expectedID'
func (p *Packet) distance(expectedID uint16, id uint16) int {
	if expectedID == id {
		return 0
	}

	// Number of packets id
	const packetIDlimit = 0x10000
	// modSubU module of subtraction
	modSubU := func(arga, argb uint16, mod uint32) int32 {
		sub := (uint32(arga) % mod) + mod - (uint32(argb) % mod)
		return int32(sub % mod)
	}

	diff := modSubU(id, expectedID, packetIDlimit)
	if diff < packetIDlimit/2 {
		return int(diff)
	}
	return int(diff - packetIDlimit)
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
