// Copyright 2024 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru packets Acks combine module.

package tru

import (
	"net"
	"sort"
	"time"
)

const (
	ackWait    = 10 * time.Millisecond // Time to wait for Acks
	acksLen    = 512                   // Maximum number of Acks collected during acksWait
	acksOutLen = 1024                  // Output asks data length
)

// Ack struct and methods to combine and send Acks
type Ack struct {
	conn net.PacketConn     // UDP connection
	comb chan *headerPacket // Channel for combining Acks
	ch   *Channel           // Tru channel
	acks []*headerPacket    // Collected acks
	t    time.Time          // First ack time
}

// newAck creates new Ack object.
func (ch *Channel) newAck(conn net.PacketConn) *Ack {
	ack := &Ack{
		conn: conn,
		comb: make(chan *headerPacket, numReaders),
		acks: make([]*headerPacket, 0, acksLen),
		ch:   ch,
	}
	go ack.process()
	return ack
}

// close closes ack processing
func (a *Ack) close() { close(a.comb) }

// conn.WriteTo(data, ch.addr)
func (a *Ack) write(data *headerPacket) { a.comb <- data }

// process receives ack packets and combines them into one long packet with 1024
// acks within 10 milliseconds.
func (a *Ack) process() {

	waitTime := ackWait

	for {
		select {
		// When an ack added
		case ack, ok := <-a.comb:
			// Stop process if channel is closed
			if !ok {
				return
			}

			// Set time of first ack added
			if a.t.IsZero() {
				a.t = time.Now()
			}

			// Add new ack to collected acks and calculate new wait time
			a.acks = append(a.acks, ack)

		// When waitTime is over
		case <-time.After(waitTime):
			// If no one acks collected during waitTime than continue collecting
			if len(a.acks) == 0 {
				continue
			}
		}

		// Calculate new wait time value
		waitTime = ackWait - time.Now().Sub(a.t)

		// Combine and Send acks to udp connection and continue collecting
		if waitTime <= 0 || len(a.acks) >= acksLen {
			if a.conn != nil { // A nil conn used in tests
				for _, data := range a.combine() {
					a.conn.WriteTo(data, a.ch.addr)
					// fmt.Println("send acks len:", len(data), "data:", data)
				}
			}

			// Clear acks slice, set default wait time and clear first packet
			// time
			a.acks = a.acks[:0]
			waitTime = ackWait
			a.t = time.Time{}
		}
	}
}

// combine combines Ack status slice packet to Acks status packet.
func (a *Ack) combine() (out [][]byte) {

	// Combines Ack and Acks status packets to slices no longer than
	// acksOutLen (1024).
	var pacs [][]byte
	defer func() { out = a.acksToBytes(pacs) }()

	// getNextId gets next id
	getNextId := func(id uint32) uint32 {
		if id++; id >= packetIDLimit {
			id = 0
		}
		return id
	}

	// Sort collected packages
	sort.Slice(a.acks, func(i, j int) bool {
		return a.acks[i].id < a.acks[j].id
	})

	// Process collected ack in a.acks slice
	var startIdx int
	var processed bool
	for !processed {
		// First and last id sets to max uint32 value before combining.
		var firstId, lastId, nextId uint32 = ^uint32(0), ^uint32(0), 0

		// The processed true mines than all ids in input data was added to the
		// output packet.
		processed = true

		// Get first and last id of acks to combine to acks status packet
		for idx := startIdx; idx < len(a.acks); idx++ {

			header := a.acks[idx]
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
			}

			if !processed {
				break
			}

			// Set last id and next expected id
			lastId = id
			nextId = getNextId(id)
		}

		// All acks combined (empty acks slice)
		if firstId == ^uint32(0) {
			return
		}

		// Add ack or acks packet to result output
		var data []byte
		if lastId == firstId {
			// Create ack packet
			var idx = startIdx
			if !processed {
				idx -= 1
			}
			data, _ = a.acks[idx].MarshalBinary()
		} else {
			// Create acks packet
			first, _ := headerPacket{uint32(firstId), pAcks}.MarshalBinary()
			last, _ := headerPacket{uint32(lastId), pAcks}.MarshalBinary()
			data = append(first, last...)
		}
		pacs = append(pacs, data)
	}

	// All acks combined
	return
}

// acksToBytes combines Ack and Acks status packets to slices no longer than
// acksOutLen (1024).
func (a *Ack) acksToBytes(pacs [][]byte) (out [][]byte) {

	outIdx := -1

	nextOutIdx := func() {
		out = append(out, make([]byte, 0, acksOutLen))
		outIdx++
	}
	nextOutIdx()

	for i := range pacs {
		if len(out[outIdx])+len(pacs[i]) > acksOutLen {
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

		first := &headerPacket{}
		err := first.UnmarshalBinary(data[idx : idx+headerLen])
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}

		end := first

		packetLen = headerLen
		if first.ptype == pAcks {

			idx = idx + headerLen
			header := &headerPacket{}
			err := header.UnmarshalBinary(data[idx : idx+headerLen])
			if err == nil {
				end = header
			}

			packetLen += headerLen
		}

		f(first, end)
	}
}
