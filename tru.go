// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU is new teonet releible udp packge
package tru

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type Tru struct {
	conn        net.PacketConn      // Local connection
	cannels     map[string]*Channel // Channels map
	reader      ReaderFunc          // Global tru reader
	readerCh    chan readerChData   // Reader channel
	senderCh    chan senderChData   // Sender channel
	connect     connect             // Connect methods receiver
	delay       int                 // Send delay
	statLogMsgs []string            // Log messages
	statTimer   *time.Timer         // Show statistic timer
	privateKey  *rsa.PrivateKey     // Common private key
	m           sync.RWMutex        // Channels map mutex
}

const chanLen = 10

var drop = flag.Int("drop", 0, "drop send packets")

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

	// Generate privater key or use private key from attr parameters
	// if param.privateKey == nil {
	//	param.privateKey, _ = GeneratePrivateKey()
	// }
	tru.privateKey, err = tru.GeneratePrivateKey()
	if err != nil {
		return
	}

	// Add global reader
	if len(reader) > 0 {
		tru.reader = reader[0]
	}

	// Start packet reader processing
	tru.readerCh = make(chan readerChData, chanLen)
	go tru.readerProccess()

	// Start packet sender processing
	tru.senderCh = make(chan senderChData, chanLen)
	go tru.senderProccess()

	// start listen to incoming udp packets
	go tru.listen()

	return
}

// Close write close to all connected channels
func (tru *Tru) Close() {
	tru.m.RLock()
	for _, ch := range tru.cannels {
		tru.m.RUnlock()
		ch.Close()
		tru.Close()
		return
	}
	tru.m.RUnlock()
	tru.conn.Close()
	tru.StopPrintStatistic()
}

func (tru *Tru) SetSendDelay(delay int) {
	tru.delay = delay
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

	// Get channel and process connection packets
	ch, ok := tru.getChannel(addr.String())
	if !ok || pac.Status() == statusConnect || pac.Status() == statusConnectClientAnswer {
		// Got connect packet from existing channel, destroy this channel first
		// becaus client reconnected
		if ok && pac.Status() == statusConnect {
			ch.destroy(fmt.Sprint("channel reconnect, destroy ", ch.addr.String()))
		}
		// Process connection packets
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

	case statusDisconnect:
		ch.destroy(fmt.Sprint("channel disconnect received, destroy ", ch.addr.String()))
		return

	case statusData, statusDataNext:
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
				pac = ch.combine.packet(pac)
				if pac != nil {
					tru.readerCh <- readerChData{ch, pac, nil}
					ch.stat.setRecv()
				}
			}
			sendToReader(ch, pac)
			ch.newExpectedID()

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

		// Check channel destroyed
		if r.ch.stat.destroyed {
			continue
		}

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
	ch  *Channel
	pac *Packet
}

// senderProccess process sended tru packets
func (tru *Tru) senderProccess() {
	for r := range tru.senderCh {

		// Check channel destroyed
		if r.ch.stat.destroyed {
			continue
		}

		// Marshal packet
		data, err := r.pac.MarshalBinary()
		if err != nil {
			return
		}

		// Write packet to addr
		tru.writeTo(data, r.ch.addr)
	}
}
