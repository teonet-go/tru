package tru

import (
	"log"
	"net"
	"sync/atomic"
	"time"
)

const (
	packetIDLimit   = 0x100000
	pingAfter       = 5000 * time.Millisecond
	pingRepeat      = 500 * time.Millisecond
	disconnectAfter = 8000 * time.Millisecond
)

// checkDisconnect checks if channels disconnected. A channel is disconnected if
// it does not receive any packets during disconnectAfter time
func (ch *Channel) checkDisconnect(conn net.PacketConn) {
	if ch.disconnected {
		return
	}

	switch {
	// Disconnect this channel
	case time.Since(ch.lastpacket()) > disconnectAfter:
		log.Printf("channel disconnected, addr: %s\n", ch.addr)
		ch.close()
		return
	// Send ping
	case time.Since(ch.lastpacket()) > pingAfter:
		if time.Since(ch.lastping) > pingRepeat {
			log.Printf("send ping packet, addr: %s\n", ch.addr)
			data, _ := headerPacket{0, pPing}.MarshalBinary()
			conn.WriteTo(data, ch.addr)
			ch.lastping = time.Now()
		}
	}

	time.AfterFunc(pingRepeat, func() { ch.checkDisconnect(conn) })
}

// Channel data structure and methods receiver
type Channel struct {
	// conn  net.PacketConn                // Network connection
	addr  net.Addr                      // Channel Remote address
	sq    *sendQueue                    // Channel Send Queue
	rq    *receiveQueue                 // Channel Receive Queue
	id    int32                         // Packet id (to send packet)
	expId int32                         // Packet expected id (to receive packet)
	att   atomic.Pointer[time.Duration] // Channel Triptime
	close func()                        // CLose this channel func

	// Statistics data:

	answer     uint64 // Number of packets answer received
	retransmit uint64 // Number of packets retransmited

	// Disconnect and ping data

	lastpac      atomic.Pointer[time.Time] // Last packet received time
	lastping     time.Time                 // Last ping packet send time
	disconnected bool                      // Disconnected flag
}

// newChannel creates new tru channel
func newChannel(conn net.PacketConn, addr string, close func()) (ch *Channel, err error) {
	ch = new(Channel)
	// ch.conn = conn
	ch.close = close
	ch.setLastpacket()
	ch.sq = newSendQueue()
	ch.rq = newReceiveQueue()
	ch.setTriptime(125 * time.Millisecond)
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

// Answer returns answer counter value
func (ch *Channel) Answer() uint64 { return atomic.LoadUint64(&ch.answer) }

// SendQueueLen returns send queue length
func (ch *Channel) SendQueueLen() int { return ch.sq.len() }

// Retransmit returns retransmit counter value
func (ch *Channel) Retransmit() uint64 { return atomic.LoadUint64(&ch.retransmit) }

// Triptime gets channel triptime
func (ch *Channel) Triptime() time.Duration { return *ch.att.Load() }

// calcTriptime calculates new timestamp
func (ch *Channel) calcTriptime(pac *sendQueueData) {
	const n = 1
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

// incAnswer increments answer counter value
func (ch *Channel) incAnswer() { atomic.AddUint64(&ch.answer, 1) }

// incAnswer increments retransmit counter value
func (ch *Channel) incRetransmit() { atomic.AddUint64(&ch.retransmit, 1) }

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
