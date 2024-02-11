// Copyright 2023 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru packets header module

package tru

import (
	"bytes"
	"encoding/binary"
)

const (

	// Basic packet types.
	// Don't move records in this list, this order is important.

	statusConnect             uint8 = iota // *** Not used in tru v6
	statusConnectServerAnswer              // *** Not used in tru v6
	statusConnectClientAnswer              // *** Not used in tru v6
	statusConnectDone                      // *** Not used in tru v6

	pData // Data packet
	pAck  // Answer to data packet (acknowledgement)
	pPing // Ping packet
	pPong // Ping answer packet

	statusDisconnect // *** Not used in tru v6
	statusPunch      // *** Not used in tru v6

	// New in tru v6 (From here to "Split packet types")

	// Combined answer to data packet (acknowledgement)
	//
	//	+--------------------+------+
	//	| ID & STATUS uint32 | DATA |
	//	+--------------------+------+
	//	- ID - first Ack id From a continuous sequence & STATUS uint32: STATUS 1 byte | ID 3 byte
	//  - DATA - last Ack id From a continuous sequence
	pAcks

	// Split packet types

	statusSplit    = 0x80                // *** Not used in tru v6
	statusDataNext = pData + statusSplit // *** Not used in tru v6
)

// headerPacket is header of tru packet struct and methods receiver
type headerPacket struct {
	id    uint32 // Packet id
	ptype uint8  // Packet type (see the packet type constants: pData, etc.)
}

// headerLen is tru binary packets header length
const headerLen = 4

// MarshalBinary marshals tru packets header
//
//	Bynary packet structure:
//	+--------------------+------+
//	| ID & STATUS uint32 | DATA |
//	+--------------------+------+
//	ID & STATUS uint32: STATUS 1 byte | ID 3 byte
func (h headerPacket) MarshalBinary() (data []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	// Combain pack status & id into uint32
	statid := h.packStatID()

	// Write combined status & id to buffer and return slice with binary header
	if err = binary.Write(buf, le, statid); err != nil {
		return
	}
	data = buf.Bytes()
	return
}

// UnmarshalBinary unmarshals tru packets header
func (h *headerPacket) UnmarshalBinary(data []byte) (err error) {
	buf := bytes.NewReader(data)
	le := binary.LittleEndian

	// Read status&id from buffer and unpack it to h.ptype and h.id
	var statid uint32
	err = binary.Read(buf, le, &statid)
	if err != nil {
		return
	}
	h.unpackStatID(statid)

	return
}

// packStatID packs status and id to uint32
func (h *headerPacket) packStatID() uint32 {
	return h.id&(packetIDLimit-1) | uint32(h.ptype)<<24
}

// unpackStatID unpacks status and id form uint32
func (h *headerPacket) unpackStatID(statid uint32) {
	h.id = statid & (packetIDLimit - 1)
	h.ptype = uint8(statid >> 24)
}
