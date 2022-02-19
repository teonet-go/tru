// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU is new teonet releible udp go packge
package tru

import (
	"crypto/rsa"
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/kirill-scherba/tru/teolog"
)

// Tru connector
type Tru struct {
	conn       net.PacketConn      // Local connection
	cannels    map[string]*Channel // Channels map
	reader     ReaderFunc          // Global tru reader
	readerCh   chan readerChData   // Reader channel
	senderCh   chan senderChData   // Sender channel
	connect    connect             // Connect methods receiver
	sendDelay  int                 // Common send delay
	statMsgs   statisticLog        // Statistic log messages
	statTimer  *time.Timer         // Show statistic timer
	privateKey *rsa.PrivateKey     // Common private key
	maxDataLen int                 // Max data len in created packets, 0 - maximum UDP len
	closed     bool                // Tru closed flag
	mu         sync.RWMutex        // Channels map mutex
}

// Lengs of readerChData and senderChData
const chanLen = 10

// Teolog
var log *teolog.Teolog

// Global flag "drop" drop send paskets for testing
var drop = flag.Int("drop", 0, "drop send packets")

// New create new tru object and start listen udp packets
func New(port int, params ...interface{} /* reader ...ReaderFunc */) (tru *Tru, err error) {

	// Create tru object
	tru = new(Tru)

	// Parse parameters
	for _, p := range params {
		switch v := p.(type) {

		// Global tru reader
		case func(ch *Channel, pac *Packet, err error) (processed bool):
			tru.reader = v
		case ReaderFunc:
			tru.reader = v

		// Teonet logger
		case *teolog.Teolog:
			log = v

		// Wrong parameter
		default:
			err = errors.New("wrong parameter")
			return
		}
	}

	// Log define
	if log == nil {
		log = teolog.New()
	}

	// Init tru object
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

	// Start packet reader processing
	tru.readerCh = make(chan readerChData, chanLen)
	go tru.readerProccess()

	// Start packet sender processing
	tru.senderCh = make(chan senderChData, chanLen)
	go tru.senderProccess()

	log.Connect.Println("tru created")

	// start listen to incoming udp packets
	go tru.listen()

	return
}

// Close close tru listner and all connected channels
func (tru *Tru) Close() {
	tru.mu.RLock()
	for _, ch := range tru.cannels {
		tru.mu.RUnlock()
		ch.Close()
		tru.Close()
		return
	}
	tru.mu.RUnlock()
	tru.closed = true
	tru.conn.Close()
	tru.StopPrintStatistic()
	log.Connect.Println("tru closed")
}

// SetSendDelay set default (start) clients send delay
func (tru *Tru) SetSendDelay(delay int) {
	tru.sendDelay = delay
}

// SetMaxDataLen set max data len in created packets, 0 - maximum UDP len
func (tru *Tru) SetMaxDataLen(maxDataLen int) {
	pac := tru.newPacket()
	l := pac.MaxDataLen()
	if maxDataLen < l {
		tru.maxDataLen = maxDataLen
	}
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
		log.Connect.Println("stop listen", tru.LocalAddr().String())
		tru.conn.Close()
	}()
	log.Connect.Println("start listen at", tru.LocalAddr().String())

	for !tru.closed {
		buf := make([]byte, 64*1024)
		n, addr, err := tru.conn.ReadFrom(buf)
		if err != nil {
			// log.Error.Println("ReadFrom error:", err)
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
		log.Debugvvv.Printf("got wrong packet %d from %s, data: %s\n", n, addr.String(), data)
		return
	}

	// Get channel and process connection packets
	ch, channelExists := tru.getChannel(addr.String())
	if !channelExists ||
		pac.Status() == statusConnect ||
		pac.Status() == statusConnectClientAnswer ||
		pac.Status() == statusConnectDone {
		// Got connect packet from existing channel, destroy this channel first
		// becaus client reconnected
		if channelExists && pac.Status() == statusConnect {
			ch.destroy(fmt.Sprint("channel reconnect, destroy ", ch.addr.String()))
		}
		// Process connection packets
		err := tru.connect.serve(tru, addr, pac)
		if channelExists && err != nil {
			ch.destroy(fmt.Sprint("channel connection error, destroy ", ch.addr.String()))
		}
		return
	}

	// Process regular packets by status
	switch pac.Status() {

	case statusPing:
		ch.writeToPong()

	case statusPong:

	case statusAck:
		tt, err := ch.setTripTime(pac.ID())
		if err != nil {
			ch.stat.setAckDropReceived()
			break
		}
		ch.stat.setAckReceived()
		log.Debugvv.Printf("got ack to packet id %d, trip time: %.3f ms", pac.ID(), float64(tt.Microseconds())/1000.0)
		pac, ok := ch.sendQueue.delete(pac.ID())
		// Execute packet delivery callback
		if delivery := pac.Delivery(); ok && delivery != nil {
			pac.deliveryTimer.Stop()
			go delivery(pac, nil)
		}

	case statusDisconnect:
		ch.destroy(fmt.Sprint("channel disconnect received, destroy ", ch.addr.String()))
		return

	case statusData, statusDataNext:
		pac.data, err = ch.decryptPacketData(pac.ID(), pac.Data())
		if err != nil {
			return
		}
		dist := pac.distance(ch.expectedID, pac.id)
		ch.writeToAck(pac)
		switch {
		// Already processed packet (id < expectedID)
		case dist < 0:
			ch.stat.setDrop()
		// Packet with id more than expectedID placed to receive queue and wait
		// previouse packets
		case dist > 0:
			_, ok := ch.recvQueue.get(pac.ID())
			if !ok {
				ch.recvQueue.add(pac)
			} else {
				ch.stat.setDrop()
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

// ReaderFunc tru reder function type
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
		if r.ch.stat.isDestroyed() {
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
		if r.ch.stat.isDestroyed() {
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
