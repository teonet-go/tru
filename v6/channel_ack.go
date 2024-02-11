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

// combineAcks combines acks
func (a *Ack) combineAcks(acks []byte, f func(*headerPacket)) {

}

func (ch *Channel) splitAcks(data []byte, f func(*headerPacket)) {
	for idx := 0; idx < len(data); idx += headerLen {

		header := &headerPacket{}
		err := header.UnmarshalBinary(data[idx : idx+headerLen])
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}

		f(header)
	}
}
