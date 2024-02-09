package tru

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"
)

const (
	// Packet ID limit from 0 to packetIDLimit-1
	packetIDLimit = 0x1000000

	// Send ping after pingAfter time if no packet received
	pingAfter = 5000 * time.Millisecond

	// Repeat pings every pingRepeat time
	pingRepeat = 500 * time.Millisecond

	// Disconnect after disconnectAfter time if nothing received
	disconnectAfter = 8000 * time.Millisecond

	// Disconnect after disconnectAfterPings * pingRepeat if only pongs recived.
	// Set to 0 for never disconnect if channels peer answer to ping.
	disconnectAfterPings = 0 // 2
)

// Channel data structure and methods receiver
type Channel struct {
	// conn  net.PacketConn             // Network connection
	addr        net.Addr                      // Channel Remote address
	sq          *sendQueue                    // Channel Send Queue
	rq          *receiveQueue                 // Channel Receive Queue
	id          int32                         // Packet id (to send packet)
	expId       int32                         // Packet expected id (to receive packet)
	att         atomic.Pointer[time.Duration] // Channel Triptime
	close       func()                        // CLose this channel func
	processChan chan processChanData          // Channel

	// Statistics data (atomic vars) and methods
	Stat ChannelStat

	// Disconnect and ping data:

	lastpac     atomic.Pointer[time.Time] // Last packet received time
	lastdatapac atomic.Pointer[time.Time] // Last Data packet or Answer received time
	lastsendpac time.Time                 // Last Data packet send time
	senddelay   time.Duration             // Send delay
	lastping    time.Time                 // Last Ping packet send time
	closedFlag  int32                     // Channel closed atomic bool flag
}

type processChanData []byte // Process channel data

// newChannel creates new tru channel
func newChannel(tru *Tru, conn net.PacketConn, addr string, close func()) (
	ch *Channel, err error) {

	if tru == nil {
		err = fmt.Errorf("tru can't be nil")
		return
	}

	var a net.Addr
	if len(addr) > 0 {
		if a, err = net.ResolveUDPAddr("udp", addr); err != nil {
			return
		}
	}
	ch = &Channel{addr: a, close: close}

	ch.Stat.init()
	ch.setLastdata()
	ch.setLastpacket()
	ch.sq = newSendQueue()
	ch.rq = newReceiveQueue()
	ch.setTriptime(0 * time.Millisecond)
	ch.senddelay = 0 * time.Microsecond
	ch.processChan = make(chan processChanData, 16)

	go ch.process(tru, conn)
	go ch.sq.process(conn, ch)
	go ch.checkDisconnect(conn)

	log.Printf("channel ctreated, addr: %s\n", ch.addr)

	return
}

// distance check received packet distance and return integer value
// lesse than zero than 'id < expectedID' or return integer value more than
// zero than 'id > tcd.expectedID'
func (ch *Channel) distance(id uint32) int {
	// Check expectedID equal to input id
	expectedID := ch.expectedId()
	if expectedID == id {
		return 0
	}

	// modSubU func returns module of subtraction
	modSubU := func(arga, argb uint32, mod uint64) int {
		sub := (uint64(arga) % mod) + mod - (uint64(argb) % mod)
		return int(sub % mod)
	}

	// check diff
	diff := modSubU(id, expectedID, packetIDLimit)
	if diff < packetIDLimit/2 {
		return int(diff)
	}
	return int(diff - packetIDLimit)
}

// SendQueueLen returns send queue length.
func (ch *Channel) SendQueueLen() int { return ch.sq.len() }

// ReceiveQueueLen returns receive queue length.
func (ch *Channel) ReceiveQueueLen() int { return ch.rq.len() }

// Triptime gets channel triptime
func (ch *Channel) Triptime() time.Duration { return *ch.att.Load() }

// calcTriptime calculates new timestamp
func (ch *Channel) calcTriptime(sqd *sendQueueData) {
	if sqd.retransmit() == 0 {
		ttCalcMiddle := time.Duration(ch.Stat.SentSpeed())
		tt := ch.Triptime()
		tt = (tt*(ttCalcMiddle) + time.Since(sqd.time())) / (ttCalcMiddle + 1)
		ch.setTriptime(tt)
	}
}

// setTriptime sets channel triptime
func (ch *Channel) setTriptime(tt time.Duration) { ch.att.Store(&tt) }

// lastpacket gets time of last packet received. The packet may be Data, Answer,
// Ping or Ping answer
func (ch *Channel) lastpacket() time.Time { return *ch.lastpac.Load() }

// setLastpacket sets last packet received time. The packet may be Data, Answer,
// Ping or Ping answer
func (ch *Channel) setLastpacket() { now := time.Now(); ch.lastpac.Store(&now) }

// lastdata gets time of last Data or Answer packet received.
func (ch *Channel) lastdata() time.Time { return *ch.lastdatapac.Load() }

// setLastdata sets last Data or Answer packet received time.
func (ch *Channel) setLastdata() { now := time.Now(); ch.lastdatapac.Store(&now) }

// expectedId gets expected id
func (ch *Channel) expectedId() uint32 { return uint32(atomic.LoadInt32(&ch.expId) % packetIDLimit) }

// newId returns new packet id
func (ch *Channel) newId() uint32 {
	// currentId func returns decremented value of new id
	currentId := func(newId int32) uint32 {
		if newId > 0 {
			return uint32(newId - 1)
		}
		return uint32(packetIDLimit - 1)
	}

	// Increment current id
	newId := atomic.AddInt32(&ch.id, 1)
	if newId < packetIDLimit {
		return currentId(newId) // We're done
	}

	// Must normalize, optionally reset:
	newId %= packetIDLimit
	if newId == 0 {
		atomic.AddInt32(&ch.id, -packetIDLimit) // Reset
	}
	return currentId(newId)
}

// newExpectedId increments, save and return new expected id
func (ch *Channel) newExpectedId() uint32 {
	// Increment current expected id
	id := atomic.AddInt32(&ch.expId, 1)
	if id < packetIDLimit {
		return uint32(id) // We're done
	}

	// Must normalize, optionally reset:
	id %= packetIDLimit
	if id == 0 {
		atomic.AddInt32(&ch.expId, -packetIDLimit) // Reset
	}
	return uint32(id)
}

// closed checks if channel already closed and return true if so
func (ch *Channel) closed() bool { return atomic.LoadInt32(&ch.closedFlag) > 0 }

// setClosed sets channel closedFlag to true
func (ch *Channel) setClosed() { atomic.StoreInt32(&ch.closedFlag, 1) }

// checkDisconnect process checks if channels disconnected. A channel is
// disconnected if it does not receive any packets during disconnectAfter time
func (ch *Channel) checkDisconnect(conn net.PacketConn) {
	// var stopProcessMsg = fmt.Sprintf(
	// 	"check disconnect process of channel %s stopped", ch.addr,
	// )
	if ch.closed() {
		// log.Println(stopProcessMsg)
		return
	}

	switch {
	// Disconnect this channel in channel does not send data and does not answer
	// to ping or
	// Disconnect if channel does not send data and ping time expired
	case time.Since(ch.lastpacket()) > disconnectAfter,
		disconnectAfterPings > 0 &&
			time.Since(ch.lastdata()) > disconnectAfter+pingAfter*disconnectAfterPings:
		// log.Println(stopProcessMsg)
		log.Printf("channel disconnected, addr: %s\n", ch.addr)
		ch.close()
		return

	// Send ping
	case time.Since(ch.lastpacket()) > pingAfter &&
		time.Since(ch.lastping) > pingRepeat:
		// log.Printf("send ping packet, addr: %s\n", ch.addr)
		data, _ := headerPacket{0, pPing}.MarshalBinary()
		conn.WriteTo(data, ch.addr)
		ch.lastping = time.Now()
	}

	time.AfterFunc(pingRepeat, func() { ch.checkDisconnect(conn) })
}

// process gets received packets from processChan and process it
func (ch *Channel) process(tru *Tru, conn net.PacketConn) (err error) {

	// writeToReadChannel func send data to read channel
	writeToReadChannel := func(data []byte, wait bool) bool {

		if wait {
			tru.readChannel <- readChannelData{ch.addr, nil, data[headerLen:]}
			ch.Stat.incRecv()
			ch.newExpectedId()
			return true
		}

		select {
		case tru.readChannel <- readChannelData{ch.addr, nil, data[headerLen:]}:
			ch.Stat.incRecv()
			ch.newExpectedId()
			return true
		default:
			return false
		}
	}

	// processReceiveQueue gets packets from receiveQueue and sends it to read channel
	processReceiveQueue := func() {
		// readChannelBusy func returns true if read channel is busy
		// readChannelBusy := func() bool { return len(tru.readChannel) >= cap(tru.readChannel) }
		for id := ch.expectedId(); ; id = ch.expectedId() {
			data, ok := ch.rq.del(id)
			if !ok {
				break
			}

			// for !writeToReadChannel(data, false) {
			// 	// can't write to read channel, restore id in receive queue
			// 	ch.rq.add(id, data)
			// 	// time.Sleep(8 * time.Microsecond)
			// 	// break
			// }

			writeToReadChannel(data, true)
		}
	}

	for data := range ch.processChan {

		// processReceiveQueue()

		// Unmarshal header
		header := headerPacket{}
		err = header.UnmarshalBinary(data)
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}

		// Set last packet received time
		ch.setLastpacket()

		// Send tru answer or process answer depend of header type
		switch header.ptype {

		// Data received
		case pData:

			// TODO: Check this code again: May be we need to lock this code
			// because we check expected id in distance func and change it befor
			// send data to readChannel in newExpectedId() func. And we have
			// some number of workers which read connection an the same time.

			var processed = true

			// Check expected id distance
			dist := ch.distance(header.id)
			switch {

			// Already processed packet (id < expectedID)
			case dist < 0:
				// Set channel drop statistic
				ch.Stat.incDrop()

			// Packet with id more than expectedID placed to receive queue and
			// wait for previouse packets
			case dist > 0:
				if err := ch.rq.add(header.id, data); err != nil {
					// Set channel drop statistic
					ch.Stat.incDrop()
				}

			// Valid data packet received (id == expectedID)
			case dist == 0:

				// Send data packet to readChannel and Process receive queue or
				// reject this data packet if read channel full
				// if !writeToReadChannel(data, false) {
				// 	processed = false
				// 	ch.setLastdata()
				// 	fmt.Println("skip", len(tru.readChannel))
				// 	break
				// }
				writeToReadChannel(data, true)
				processReceiveQueue()
			}

			// Send answer
			if processed {
				data, _ := headerPacket{header.id, pAck}.MarshalBinary()
				conn.WriteTo(data, ch.addr)
				ch.setLastdata()
			}

		// Answer to data packet (acknowledgement) received
		case pAck:
			// Save answer statistic, calculate triptime and remove package from
			// send queue
			ch.setLastdata()

			if pac, ok := ch.sq.del(header.id); ok {
				ch.calcTriptime(pac)
				ch.Stat.incAck()
			} else {
				ch.Stat.incAckd()
			}

		// Ping received
		case pPing:
			data, _ := headerPacket{0, pPong}.MarshalBinary()
			conn.WriteTo(data, ch.addr)

		// pPong (ping answer) received
		case pPong:
		}
	}

	return
}
