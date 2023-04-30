// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU is new teonet releible udp go package
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

	"github.com/teonet-go/tru/hotkey"
	"github.com/teonet-go/tru/teolog"
)

const truName = "Teonet Reliable UDP (TRU v5)"
const truVersion = "0.0.11"

// Tru connector
type Tru struct {
	conn       net.PacketConn      // Local connection
	channels   map[string]*Channel // Channels map
	reader     ReaderFunc          // Global tru reader callback
	punchcb    PunchFunc           // Punch packet callback
	connectcb  ConnectFunc         // Connect to this server callback
	readerCh   chan readerChData   // Reader channel
	senderCh   chan senderChData   // Sender channel
	connect    connect             // Connect methods receiver
	sendDelay  int                 // Common send delay
	statMsgs   statisticLog        // Statistic log messages
	statTimer  *time.Timer         // Show statistic timer
	start      time.Time           // Start time
	privateKey *rsa.PrivateKey     // Common private key
	maxDataLen int                 // Max data len in created packets, 0 - maximum UDP len
	listenStop chan interface{}    // Tru listen wait stop channel
	hotkey     *hotkey.Hotkey      // Hotkey menu
	mu         sync.RWMutex        // Channels map mutex
}

type Stat bool          // Parameters show statistic type
type Hotkey bool        // Parameters start hotkey menu
type MaxDataLenType int // Max data length type

// Lengs of readerChData and senderChData
const (
	chanLen        = 10
	startSendDelay = 15 // 250
)

// Teolog
var log *teolog.Teolog

// Global flag "drop" drop send paskets for testing
var drop = flag.Int("drop", 0, "drop send packets")

var ErrTruClosed = errors.New("tru listner closed")

// New create new tru object and start listen udp packets. Parameters by type:
//
//	int:                local port number, 0 for any
//	tru.ReaderFunc:     message receiver callback function
//	tru.ConnectFunc:    connect to server callback function
//	tru.PunchFunc:      punch callback function
//	*teolog.Teolog:     pointer to teolog
//	string:             loggers level
//	teolog.Filter:      loggers filter
//	tru.StartHotkey:    start hotkey meny
//	tru.ShowStat:       show statistic
//	tru.MaxDataLenType: max packet data length
func New(port int, params ...interface{}) (tru *Tru, err error) {

	// Create tru object
	tru = new(Tru)
	tru.start = time.Now()
	tru.sendDelay = startSendDelay

	// Parse parameters
	var logFilter teolog.Filter
	var logLevel string
	for _, p := range params {
		switch v := p.(type) {

		// Global tru reader
		case func(ch *Channel, pac *Packet, err error) (processed bool):
			tru.reader = v
		case ReaderFunc:
			tru.reader = v

		// Connect to this server callback
		case func(*Channel, error):
			tru.connectcb = v
		case ConnectFunc:
			tru.connectcb = v

		// Connect to this server callback
		case func(net.Addr, []byte):
			tru.punchcb = v
		case PunchFunc:
			tru.punchcb = v

		// Teonet logger
		case *teolog.Teolog:
			log = v

		// Teonet loggers level
		case string:
			logLevel = v

		// Teonet loggers filter
		case teolog.Filter:
			logFilter = v

		// Start hokey menu
		case Hotkey:
			if v {
				tru.hotkey = tru.newHotkey()
			}

		// Show statistic
		case Stat:
			if v {
				tru.StatisticPrint()
			}

		// Private key
		case *rsa.PrivateKey:
			// TODO: set private key

		// Set max data length
		case MaxDataLenType:
			tru.maxDataLen = int(v)

		// Wrong parameter
		default:
			err = fmt.Errorf("incorrect attribute type '%T'", v)
			return
		}
	}

	// Log define and set filter
	if log == nil {
		log = teolog.New()
	}
	log.SetFilter(logFilter)
	log.SetLevel(logLevel)

	// Init tru object
	tru.listenStop = make(chan interface{})
	tru.channels = make(map[string]*Channel)
	tru.connect.connects = make(map[string]*connectData)
	tru.conn, err = net.ListenPacket("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return
	}

	// Generate privater key or use private key from attr parameters
	// if param.privateKey == nil {
	//	param.privateKey, _ = GeneratePrivateKey()
	// }
	tru.privateKey, err = GeneratePrivateKey()
	if err != nil {
		return
	}

	// Start packet reader processing
	tru.readerCh = make(chan readerChData, chanLen)
	go tru.readerProccess()

	// Start packet sender processing
	tru.senderCh = make(chan senderChData, chanLen)
	go tru.senderProccess()

	// start listen to incoming udp packets
	go tru.listen()

	log.Connect.Println("tru created")

	return
}

// Close tru listner and all connected channels
func (tru *Tru) Close() {

	// Send error message to channel reader
	if tru.reader != nil {
		tru.reader(nil, nil, ErrTruClosed)
	}

	// Close all channels
	log.Debug.Println("close all channels")
	tru.ForEachChannel(func(ch *Channel) { ch.Close() })

	// Stop listner and statistic
	tru.stopListen()
	tru.StatisticPrintStop()
	log.Connect.Println("tru closed")
}

// SetPunchCb set punch callback
func (tru *Tru) SetPunchCb(punchcb PunchFunc) {
	tru.punchcb = punchcb
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

// LocalPort returns the local network port
func (tru *Tru) LocalPort() int {
	return tru.LocalAddr().(*net.UDPAddr).Port
}

// writeTo writes a packet with data to an UDP address (unreliable write to UDP)
func (tru *Tru) WriteTo(data []byte, addri interface{}) (addr net.Addr, err error) {

	// Resolve UDP address
	// var addr *net.UDPAddr
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

// Hotkey return pointer to hotkey menu used in tru or nil if hotkey menu does
// not start
func (tru *Tru) Hotkey() *hotkey.Hotkey { return tru.hotkey }

// listen to incoming udp packets
func (tru *Tru) listen() {
	log.Connect.Println("start listen at", tru.LocalAddr().String())

	for {
		select {

		// Check channel closed to stop listen and return
		case _, ok := <-tru.listenStop:
			if !ok {
				log.Debug.Println("stop listen", tru.LocalAddr().String())
				return
			}

		// Read data from tru connect (from UDP port)
		default:
			buf := make([]byte, 64*1024)
			n, addr, err := tru.conn.ReadFrom(buf)
			if err != nil {
				// log.Error.Println("ReadFrom error:", err)
				continue
			}
			if n > 0 {
				tru.serve(n, addr, buf[:n])
			}
		}
	}
}

// stopListen stop listen to incoming udp packets
func (tru *Tru) stopListen() {
	close(tru.listenStop) // close listen wait channel to stop listen
	tru.conn.Close()
}

// serve received packet
func (tru *Tru) serve(n int, addr net.Addr, data []byte) {

	// Unmarshal packet
	pac := tru.newPacket()
	err := pac.UnmarshalBinary(data)
	if err != nil {
		// Wrong packet received from addr
		log.Error.Printf("got wrong packet %d from %s, data: %s\n", n, addr.String(), data)
		return
	}

	// Get channel and process connection packets
	ch, channelExists := tru.getChannel(addr.String())

	// Process connect or punc packets
	// if !channelExists ||
	// 	pac.Status() == statusConnect ||
	// 	pac.Status() == statusConnectClientAnswer ||
	// 	pac.Status() == statusConnectDone {
	// 	// Got connect packet from existing channel, destroy this channel first
	// 	// becaus client reconnected
	// 	if channelExists && pac.Status() == statusConnect {
	// 		ch.destroy(fmt.Sprint("channel reconnect, destroy ", ch.addr.String()))
	// 	}
	// 	// Process connection packets
	// 	err := tru.connect.serve(tru, addr, pac)
	// 	if channelExists && err != nil {
	// 		ch.destroy(fmt.Sprint("channel connection error, destroy ", ch.addr.String()))
	// 	}
	// 	return
	// }

	// Process connect or punc packets
	switch pac.Status() {

	// Connect packets
	case statusConnect, statusConnectServerAnswer, statusConnectClientAnswer, statusConnectDone:
		// When got connect packet from existing channel we destroy this channel
		// first becaus client reconnected
		if channelExists && pac.Status() == statusConnect {
			ch.destroy(fmt.Sprint("channel reconnect, destroy ", ch.addr.String()))
		}
		// Process connection packets
		err := tru.connect.serve(tru, addr, pac)
		if channelExists && err != nil {
			ch.destroy(fmt.Sprint("channel connection error, destroy ", ch.addr.String()))
		}
		return

	// Punch packets: hi level software (f.e. teonet package) use punch packets
	// to make p2p connection between tru clients
	case statusPunch:
		if tru.punchcb != nil {
			tru.punchcb(addr, pac.Data())
		}
		return

	// Wrong packets: some other packets received when channel does not exists
	default:
		if !channelExists {
			return
		}
	}

	// Process regular packets by status
	switch pac.Status() {

	case statusPing:
		log.Debugvvv.Println("got ping", ch)
		ch.writeToPong()

	case statusPong:
		log.Debugvvv.Println("got ping answer", ch)

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
				ch.newExpectedID()
				pac = ch.combine.packet(pac)
				if pac == nil {
					return
				}
				tru.readerCh <- readerChData{ch, pac, nil}
				ch.stat.setRecv()
			}
			sendToReader(ch, pac)
			// ch.newExpectedID()

			ch.recvQueue.process(ch, sendToReader)
		}
	}

	ch.stat.setLastActivity()
}

// ReaderFunc tru reder function type
type ReaderFunc func(ch *Channel, pac *Packet, err error) (processed bool)

// ConnectFunc connect to server function type
type ConnectFunc func(*Channel, error)

// PunchFunc puch packet function type
type PunchFunc func(addr net.Addr, data []byte)

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
		tru.WriteTo(data, r.ch.addr)
	}
}

// Logo return tru logo in string format
func Logo(appName, appVersion string) (str string) {
	const defName = "Sample application"
	const defVersion = "0.0.1"
	const logo = `
 _____ ____  _   _  
|_   _|  _ \| | | | v5
  | | | |_) | | | |
  | | |  _ <| |_| |
  |_| |_| \_\\___/ 
`
	if len(appName) == 0 {
		appName = defName
	}
	if len(appVersion) == 0 {
		appVersion = defVersion
	}
	str += logo
	str += fmt.Sprintf("\n%s ver %s, based on %s ver %s\n",
		appName, appVersion, truName, truVersion)

	return
}
func (tru *Tru) Logo(appName, appVersion string) (str string) {
	return Logo(appName, appVersion)
}
