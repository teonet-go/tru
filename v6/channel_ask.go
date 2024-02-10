// Copyright 2024 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru packets Asks combine module.

package tru

import (
	"net"
	"time"
)

const (
	askWait = 10 * time.Millisecond // Time to wait for Asks
	asksLen = 1024                  // Maximum number of Asks: 1024 bytes / headerLen
)

// Ask struct and methods to combine and send Asks
type Ask struct {
	conn net.PacketConn // UDP connection
	comb chan []byte    // Channel for combining Asks
	ch   *Channel       // Tru channel
	asks []byte         // Combined asks
	t    time.Time      // First ask time
}

// newAsk creates new Ask object.
func (ch *Channel) newAsk(conn net.PacketConn) *Ask {
	ask := &Ask{
		conn: conn,
		comb: make(chan []byte, numReaders),
		asks: make([]byte, 0, asksLen),
		ch:   ch,
	}
	go ask.process()
	return ask
}

// close closes ask processing
func (a *Ask) close() { close(a.comb) }

// conn.WriteTo(data, ch.addr)
func (a *Ask) write(data []byte) {
	// a.conn.WriteTo(data, a.ch.addr)
	a.comb <- data
}

// process receives ask packets and combines them into one long packet with 1024
// asks within 10 milliseconds.
func (a *Ask) process() {

	waitTime := askWait

next:
	// fmt.Println("ask waitTime:", waitTime)
	select {
	case ask, ok := <-a.comb:
		if !ok {
			// fmt.Println("channel is closed")
			return
		}

		// Set time of first ask added
		if a.t.IsZero() {
			// fmt.Println("got first packet")
			a.t = time.Now()
		}

		// Add new ask to combined asks and calculate new wait time
		a.asks = append(a.asks, ask...)

	case <-time.After(waitTime):
		// fmt.Println("got time after, a.asks:", len(a.asks)/headerLen)
		if len(a.asks) == 0 {
			goto next
		}
	}

	// New wait time
	waitTime = askWait - time.Now().Sub(a.t)

	// Send asks to udp connection and continue waiting
	if waitTime <= 0 || len(a.asks) >= asksLen {
		if a.conn != nil {
			a.conn.WriteTo(a.asks, a.ch.addr)
			// fmt.Printf("send %d asks \n", len(a.asks)/headerLen)
		} else {
			// fmt.Printf("send %d asks \n", len(a.asks)/headerLen)
		}
		a.asks = a.asks[:0]
		waitTime = askWait
		a.t = time.Time{}
	}
	goto next
}

func (ch *Channel) splitAsks(data []byte, f func(*headerPacket)) {
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
