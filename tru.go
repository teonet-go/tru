// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU is new teonet releible udp packge
package tru

import (
	"log"
	"net"
	"strconv"
	"sync"
)

type Tru struct {
	conn     net.PacketConn      // Local connection
	cannels  map[string]*Channel // Channels map
	reader   ReaderFunc          // Global tru reader
	readerCh chan readerChData   // Reader channel
	senderCh chan senderChData   // Sender channel
	connect  connect             // Connect methods receiver
	m        sync.RWMutex        // Channels map mutex
}

// New create new tru object and start listen udp packets
func New(port int, reader ...ReaderFunc) (tru *Tru, err error) {

	// Create tru object
	tru = new(Tru)
	tru.cannels = make(map[string]*Channel)
	tru.connect.connects = make(map[string]*connectData)
	tru.conn, err = net.ListenPacket("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return
	}

	// Add global reader
	if len(reader) > 0 {
		tru.reader = reader[0]
	}

	// Start packet reader processing
	tru.readerCh = make(chan readerChData, 10)
	go tru.readerProccess()

	// Start packet sender processing
	tru.senderCh = make(chan senderChData, 10)
	go tru.senderProccess()

	// start listen to incoming udp packets
	go tru.listen()

	return
}

// LocalAddr returns the local network address
func (tru *Tru) LocalAddr() net.Addr {
	return tru.conn.LocalAddr()
}

// writeTo writes a packet with data to addr
func (tru *Tru) writeTo(data []byte, addri interface{}) (err error) {

	// Resolve UDP address
	var addr *net.UDPAddr
	switch v := addri.(type) {
	case string:
		addr, err = net.ResolveUDPAddr("udp", v)
		if err != nil {
			return
		}
	case *net.UDPAddr:
		addr = v
	}

	// Write data to addr
	tru.conn.WriteTo(data, addr)

	return
}

// listen to incoming udp packets
func (tru *Tru) listen() {
	defer func() {
		log.Printf("stop listen\n")
		tru.conn.Close()
	}()
	log.Printf("start listen at %s\n", tru.LocalAddr().String())

	for {
		buf := make([]byte, 64*1024)
		n, addr, err := tru.conn.ReadFrom(buf)
		if err != nil {
			continue
		}

		tru.serve(n, addr, buf[:n])
	}
}

// serve received packet
func (tru *Tru) serve(n int, addr net.Addr, data []byte) {

	// Unmarshal packet
	pac := tru.newPacket()
	err := pac.UnmarshalBinary(data)
	if err != nil {
		// Wrong packet received from addr
		log.Printf("got wrong packet %d from %s, data: %s\n", n, addr.String(), data)
		return
	}

	// Get channel
	ch, ok := tru.getChannel(addr.String())
	if !ok || pac.Status() == statusConnect {
		// Got packet from new channel
		if ok {
			ch.destroy()
		}
		tru.connect.serve(tru, addr, pac)
		return
	}

	switch pac.Status() {

	case statusPing:
		ch.writeToPong()

	case statusPong:

	case statusAck:
		tt, err := ch.setTripTime(pac.ID())
		if err != nil {
			break
		}
		log.Printf("got ack to packet id %d, trip time: %.3f ms", pac.ID(), float64(tt.Microseconds())/1000.0)
		ch.sendQueue.delete(pac.ID())

	case statusData:
		dist := pac.distance(ch.expectedID, pac.id)
		ch.writeToAck(pac)
		switch {
		// Already processed packet (id < expectedID)
		case dist < 0:
			ch.stat.drop++
		// Packet with id more than expectedID placed to receive queue and wait
		// previouse packets
		case dist > 0:
			_, ok := ch.recvQueue.get(pac.ID())
			if !ok {
				ch.recvQueue.add(pac)
			} else {
				ch.stat.drop++
			}
		// Valid data packet received (id == expectedID)
		case dist == 0:
			// Send packet to reader process and process receive queue
			sendToReader := func(ch *Channel, pac *Packet) {
				tru.readerCh <- readerChData{ch, pac, nil}
				ch.newExpectedID()
				ch.stat.recv++
			}
			sendToReader(ch, pac)
			ch.recvQueue.process(ch, sendToReader)
		}
	}

	ch.stat.setLastActivity()
}

type ReaderFunc func(ch *Channel, pac *Packet, err error) (processed bool)

type readerChData struct {
	ch  *Channel
	pac *Packet
	err error
}

// readerProccess process received tru packets
func (tru *Tru) readerProccess() {
	for r := range tru.readerCh {

		// Execute channel reader
		if r.ch.reader != nil {
			if r.ch.reader(r.ch, r.pac, nil) {
				continue
			}
		}

		// Execute global reader
		if tru.reader != nil {
			tru.reader(r.ch, r.pac, nil)
		}
	}
}

type senderChData struct {
	ch     *Channel
	data   []byte
	status int
	id     int
}

// senderProccess process sended tru packets
func (tru *Tru) senderProccess() {
	for r := range tru.senderCh {

		if r.ch.stat.destroyed {
			continue
		}

		// Create and marshal packet
		id := r.id
		if r.status == statusData {
			id = r.ch.newID()
		}
		pac := tru.newPacket().SetID(id).SetStatus(r.status).SetData(r.data)
		data, err := pac.MarshalBinary()
		if err != nil {
			return
		}

		// Add data packet to send queue and Set packet retransmit time
		if r.status == statusData {
			r.ch.setRetransmitTime(pac)
			r.ch.sendQueue.add(pac)
			r.ch.stat.send++
		}

		// Write packet to addr
		// tru.conn.WriteTo(pac, r.ch.Addr)
		tru.writeTo(data, r.ch.addr)
	}
}
