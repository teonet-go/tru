// Copyright 2024 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru packets Acks combine module.

package tru

import (
	"net"
	"time"
)

const (
	ackWait = 10 * time.Millisecond // Time to wait for Acks
	acksLen = 1024                  // Maximum number of Acks: 1024 bytes / headerLen
)

// Ack struct and methods to combine and send Acks
type Ack struct {
	conn net.PacketConn // UDP connection
	comb chan []byte    // Channel for combining Acks
	ch   *Channel       // Tru channel
	acks []byte         // Combined acks
	t    time.Time      // First ack time
}

// newAck creates new Ack object.
func (ch *Channel) newAck(conn net.PacketConn) *Ack {
	ack := &Ack{
		conn: conn,
		comb: make(chan []byte, numReaders),
		acks: make([]byte, 0, acksLen),
		ch:   ch,
	}
	go ack.process()
	return ack
}

// close closes ack processing
func (a *Ack) close() { close(a.comb) }

// conn.WriteTo(data, ch.addr)
func (a *Ack) write(data []byte) {
	// a.conn.WriteTo(data, a.ch.addr)
	a.comb <- data
}

// process receives ack packets and combines them into one long packet with 1024
// acks within 10 milliseconds.
func (a *Ack) process() {

	waitTime := ackWait

next:
	// fmt.Println("ack waitTime:", waitTime)
	select {
	case ack, ok := <-a.comb:
		if !ok {
			// fmt.Println("channel is closed")
			return
		}

		// Set time of first ack added
		if a.t.IsZero() {
			// fmt.Println("got first packet")
			a.t = time.Now()
		}

		// Add new ack to combined acks and calculate new wait time
		a.acks = append(a.acks, ack...)

	case <-time.After(waitTime):
		// fmt.Println("got time after, a.acks:", len(a.acks)/headerLen)
		if len(a.acks) == 0 {
			goto next
		}
	}

	// New wait time
	waitTime = ackWait - time.Now().Sub(a.t)

	// Send acks to udp connection and continue waiting
	if waitTime <= 0 || len(a.acks) >= acksLen {
		if a.conn != nil {
			a.conn.WriteTo(a.acks, a.ch.addr)
			// fmt.Printf("send %d acks \n", len(a.acks)/headerLen)
		} else {
			// fmt.Printf("send %d acks \n", len(a.acks)/headerLen)
		}
		a.acks = a.acks[:0]
		waitTime = ackWait
		a.t = time.Time{}
	}
	goto next
}

// combineAcks combines Ack status slice packet to Acks status packet.
func (a *Ack) combineAcks(data []byte) (pacs [][]byte) {

	// getNextId gets next id
	getNextId := func(id uint32) uint32 {
		id++
		if !(id < packetIDLimit) {
			id = 0
		}
		return uint32(id)
	}

	var startIdx int

next:
	// First id sets to max uint32 value before combining.
	var firstId, lastId, nextId uint32 = ^uint32(0), ^uint32(0), 0

	// The processed true mines than all ids in input data was added to the
	// output packet.
	var processed = true

loop:
	for idx := startIdx; idx < len(data); idx += headerLen {

		header := &headerPacket{}
		err := header.UnmarshalBinary(data[idx : idx+headerLen])
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}
		id := header.id

		switch {

		// Get first id
		case firstId == ^uint32(0):
			firstId = id

		// Next id is in range
		case id == nextId:

		// End processing
		default:
			processed = false
			startIdx = idx
			break loop
		}

		// Set last id and next expected id
		lastId = id
		nextId = getNextId(id)
	}

	// All processed
	if firstId == ^uint32(0) {
		return
	}

	// Add ack or acks packet to result output
	if lastId == firstId {
		// Create ack packet
		pacs = append(pacs, data[startIdx-headerLen:startIdx])
	} else {
		// Create acks packet
		first, _ := headerPacket{uint32(firstId), pAcks}.MarshalBinary()
		last, _ := headerPacket{uint32(lastId), pAcks}.MarshalBinary()
		data := append(first, last...)
		pacs = append(pacs, data)
	}

	// Continue processing
	if !processed {
		goto next
	}

	return
}

// acksToBytes combines Ack and Acks status packets to slices no longer than
// acksLen (1024).
func (a *Ack) acksToBytes(pacs [][]byte) (out [][]byte) {

	outIdx := -1

	nextOutIdx := func() {
		out = append(out, make([]byte, 0, acksLen))
		outIdx++
	}
	nextOutIdx()

	for i := range pacs {
		if len(out[outIdx])+len(pacs[i]) > acksLen {
			nextOutIdx()
		}
		out[outIdx] = append(out[outIdx], pacs[i]...)
	}

	return
}

// acksToBytesLen calculates length of Ack and Acks status packets.
func (a *Ack) acksToBytesLen(out [][]byte) (l int) {
	for _, p := range out {
		l += len(p)
	}
	return
}

// splitAcks splits input data to pAck or pAcks status packets and call f
// callback function for each packet.
func (ch *Channel) splitAcks(data []byte, f func(*headerPacket, *headerPacket)) {
	var packetLen int
	for idx := 0; idx < len(data); idx += headerLen {

		header := &headerPacket{}
		err := header.UnmarshalBinary(data[idx : idx+headerLen])
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}

		end := header

		packetLen = headerLen
		if header.ptype == pAcks {
			idx := idx + headerLen
			header := &headerPacket{}
			err := header.UnmarshalBinary(data[idx : idx+headerLen])
			if err == nil {
				end = header
			}

			packetLen += headerLen
		}

		f(header, end)
	}
}
