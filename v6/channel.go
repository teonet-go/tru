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
	packetIDLimit = 0x100000

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
	addr  net.Addr                      // Channel Remote address
	sq    *sendQueue                    // Channel Send Queue
	rq    *receiveQueue                 // Channel Receive Queue
	id    int32                         // Packet id (to send packet)
	expId int32                         // Packet expected id (to receive packet)
	att   atomic.Pointer[time.Duration] // Channel Triptime
	close func()                        // CLose this channel func

	// Statistics data (atomic vars) and methods
	Stat ChannelStat

	// Disconnect and ping data:

	lastpac     atomic.Pointer[time.Time] // Last packet received time
	lastdatapac atomic.Pointer[time.Time] // Last Data packet or Answer received time
	lastping    time.Time                 // Last Ping packet send time
	closedFlag  int32                     // Channel closed atomic bool flag
}

// ChannelStat contains statistics data (atomic vars) and methods to work with it
type ChannelStat struct {
	sent       uint64 // Number of data packets sent
	ack        uint64 // Number of packets answer (acknowledgement) received
	ackd       uint64 // Number of dropped packets answer (acknowledgement)
	retransmit uint64 // Number of packets data retransmited
	recv       uint64 // Number of data packets received
	drop       uint64 // Number of dropped received data packets
}

// newChannel creates new tru channel
func newChannel(conn net.PacketConn, addr string, close func()) (ch *Channel, err error) {
	ch = new(Channel)
	ch.setLastdata()
	ch.close = close
	ch.setLastpacket()
	ch.sq = newSendQueue()
	ch.rq = newReceiveQueue()
	ch.setTriptime(1 * time.Millisecond)
	if len(addr) > 0 {
		ch.addr, err = net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return
		}
	}
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

// Ack returns answer (acknowledgement) counter value
func (ch *ChannelStat) Ack() uint64 { return atomic.LoadUint64(&ch.ack) }

// ackd returns dropped answer (acknowledgement) counter value
func (ch *ChannelStat) Ackd() uint64 { return atomic.LoadUint64(&ch.ackd) }

// Sent returns data packet sent counter value
func (ch *ChannelStat) Sent() uint64 { return atomic.LoadUint64(&ch.sent) }

// Recv returns data packet received counter value
func (ch *ChannelStat) Recv() uint64 { return atomic.LoadUint64(&ch.recv) }

// Drop returns number of droppet received data packets (duplicate data packets)
func (ch *ChannelStat) Drop() uint64 { return atomic.LoadUint64(&ch.drop) }

// SendQueueLen returns send queue length
func (ch *Channel) SendQueueLen() int { return ch.sq.len() }

// Retransmit returns retransmit counter value
func (ch *ChannelStat) Retransmit() uint64 { return atomic.LoadUint64(&ch.retransmit) }

// Triptime gets channel triptime
func (ch *Channel) Triptime() time.Duration { return *ch.att.Load() }

// calcTriptime calculates new timestamp
func (ch *Channel) calcTriptime(pac *sendQueueData) {
	const n = 10
	tt := ch.Triptime()
	tt = (tt*(n-1) + time.Since(pac.time())) / n
	ch.setTriptime(tt)
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

// incAck increments answers (acknowledgement) counter value
func (ch *ChannelStat) incAck() { atomic.AddUint64(&ch.ack, 1) }

// incAckd increments dropped answers (acknowledgement) counter value
func (ch *ChannelStat) incAckd() { atomic.AddUint64(&ch.ackd, 1) }

// incSent increments send data peckets counter value
func (ch *ChannelStat) incSent() { atomic.AddUint64(&ch.sent, 1) }

// incRecv increments received data peckets counter value
func (ch *ChannelStat) incRecv() { atomic.AddUint64(&ch.recv, 1) }

// incDrop increments dropped data peckets counter value
func (ch *ChannelStat) incDrop() { atomic.AddUint64(&ch.drop, 1) }

// incAnswer increments retransmit counter value
func (ch *ChannelStat) incRetransmit() { atomic.AddUint64(&ch.retransmit, 1) }

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
	case time.Since(ch.lastpacket()) > disconnectAfter, disconnectAfterPings > 0 &&
		time.Since(ch.lastdata()) > disconnectAfter+pingAfter*disconnectAfterPings:
		// log.Println(stopProcessMsg)
		log.Printf("channel disconnected, addr: %s\n", ch.addr)
		for id := range ch.rq.m {
			fmt.Println(id)
		}
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
