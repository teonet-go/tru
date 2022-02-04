// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Packet module

package tru

import (
	"bytes"
	"encoding/binary"
)

type Packet struct {
	id     uint16 // Packet ID
	status uint8  // Packet Type
	data   []byte // Packet Data
}

const (
	statusConnect = iota
	statusConnectAnswer
	statusData
	statusAck
	statusPing
	statusDisconnect
)

// newPacket create new empty packet
func (tru *Tru) newPacket() *Packet {
	return &Packet{}
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
