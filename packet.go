// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Packet module

package tru

import (
	"bytes"
	"encoding/binary"
	"time"
)

type Packet struct {
	id                 uint16    // Packet ID
	status             uint8     // Packet Type
	data               []byte    // Packet Data
	time               time.Time // Packet creating time
	retransmitTime     time.Time // Packet retransmit time
	retransmitAttempts int       // Packet retransmit attempts
}

const (
	statusConnect = iota
	statusConnectAnswer
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
)

// newPacket create new empty packet
func (tru *Tru) newPacket() *Packet {
	return &Packet{time: time.Now()}
}

// MarshalBinary
func (p *Packet) MarshalBinary() (out []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	binary.Write(buf, le, p.id)
	binary.Write(buf, le, p.status)
	binary.Write(buf, le, p.data)

	out = buf.Bytes()
	return
}

// UnmarshalBinary
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
	return maxUdpDataLength - p.HeaderLen()
	// return 512 // look like optimal packet data length 
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
func (p *Packet) SetData(data []byte) (out *Packet) {
	out = p
	out.data = data
	return
}

// ID get packet id
func (p *Packet) ID() int {
	return int(p.id)
}

// SetID set packet id
func (p *Packet) SetID(id int) (out *Packet) {
	out = p
	out.id = uint16(id)
	return
}

// Status get packet status
func (p *Packet) Status() int {
	return int(p.status)
}

// SetStatus set packet status
func (p *Packet) SetStatus(status int) (out *Packet) {
	out = p
	out.status = uint8(status)
	return
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
