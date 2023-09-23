package tru

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const errCantCreateChannel = "can't create tru channel: %s"

const (
	readChannelLen    = 4 * 1024
	readChannelBufLen = 2 * 1024
	numReaders        = 4
)

// Tru is main tru data structure and methods reciever
type Tru struct {
	channels      // Channels map
	*sync.RWMutex // Channels map mutex
	readChannel   chan readChannelData

	started time.Time // Tru started time
}
type readChannelData struct {
	n    int
	addr net.Addr
	err  error
	data []byte
}
type channels map[string]*Channel

// New creates new Tru object
func New(printStat bool) *Tru {
	tru := new(Tru)
	tru.started = time.Now()
	tru.channels = make(channels)
	tru.RWMutex = new(sync.RWMutex)
	tru.readChannel = make(chan readChannelData, readChannelLen)
	if printStat {
		tru.printstat()
	}
	return tru
}

// ListenPacket announces on the local network address.
//
// For UDP and IP networks, if the host in the address parameter is
// empty or a literal unspecified IP address, ListenPacket listens on
// all available IP addresses of the local system except multicast IP
// addresses.
// To only use IPv4, use network "udp4" or "ip4:proto".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The LocalAddr method of PacketConn can be used to discover the
// chosen port.
//
// See func Dial for a description of the network and address
// parameters.
func (tru *Tru) ListenPacket(network, address string) (net.PacketConn, error) {
	conn, err := net.ListenPacket(network, address)
	truConn := &truPacketConn{conn: conn, tru: tru}
	for i := 0; i < numReaders; i++ {
		go truConn.readFrom()
	}
	return truConn, err
}

// GetChannel gets Tru channel by addr
func (tru *Tru) GetChannel(addr net.Addr) *Channel {
	tru.RLock()
	defer tru.RUnlock()

	ch, ok := tru.channels[addr.String()]
	if !ok {
		return nil
	}

	return ch
}

// newChannel creates new Tru channel with addr or return existing
func (tru *Tru) newChannel(conn net.PacketConn, addr net.Addr) (ch *Channel, err error) {
	tru.Lock()
	defer tru.Unlock()

	// Get string address
	addrStr := addr.String()

	// CHeck channel exists and return existing channel
	if ch, ok := tru.channels[addrStr]; ok {
		return ch, nil
	}

	// Create new channel and add it to channels map
	ch, err = newChannel(conn, addrStr, func() { tru.delChannel(ch) })
	if err != nil {
		return nil, err
	}
	tru.channels[addrStr] = ch

	return
}

// delChannel removes selected Tru channel or return error if does not exist
func (tru *Tru) delChannel(ch *Channel) error {
	tru.Lock()
	defer tru.Unlock()

	addr := ch.addr.String()
	if _, ok := tru.channels[addr]; !ok {
		return fmt.Errorf("channel does not exists")
	}

	delete(tru.channels, addr)
	ch.setClosed()
	return nil
}

// getFromReceiveQueue gets saved packet with expected id from receive queue on
// any channel
func (tru *Tru) getFromReceiveQueue(p []byte) (n int, addr net.Addr, err error) {
	tru.RLock()
	defer tru.RUnlock()

	for _, ch := range tru.channels {
		data, ok := ch.rq.process(ch)
		if ok {
			addr = ch.addr
			n = copy(p, data)
			return
		}
	}
	err = fmt.Errorf("packet not found")
	return
}

// truPacketConn is a generic packet-oriented network connection.
//
// Multiple goroutines may invoke methods on a PacketConn simultaneously.
type truPacketConn struct {
	conn net.PacketConn
	tru  *Tru
}

// ReadFrom reads a packet from the connection,
// copying the payload into p. It returns the number of
// bytes copied into p and the return address that
// was on the packet.
// It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered. Callers should always process
// the n > 0 bytes returned before considering the error err.
// ReadFrom can be made to time out and return an error after a
// fixed time limit; see SetDeadline and SetReadDeadline.
func (c *truPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	// Read processed message from read go channel
	r := <-c.tru.readChannel
	n, addr, err = r.n, r.addr, r.err
	copy(p, r.data)
	return
}

// readFrom is reader worker
func (c *truPacketConn) readFrom( /* p []byte */ ) ( /* n int, addr net.Addr, */ err error) {
	p := make([]byte, readChannelBufLen)
	for {

		var n int
		var addr net.Addr
		var readChannelBusy bool = len(c.tru.readChannel) >= cap(c.tru.readChannel)

		// Get saved packet with expected id from receive queue on any channel
		var gotFromReceiveQueue bool
		if !readChannelBusy {
			n, addr, err = c.tru.getFromReceiveQueue(p)
			if err == nil {
				gotFromReceiveQueue = true
			}
		}

		// Read data from connection
		if !gotFromReceiveQueue {
			n, addr, err = c.conn.ReadFrom(p)
			if err != nil {
				log.Println("read from error:", err)
				return
			}
		}

		// Unmarshal header
		header := headerPacket{}
		err = header.UnmarshalBinary(p)
		if err != nil {
			// TODO: print some message if wrong header received?
			continue
		}

		// Get or create channel
		var ch *Channel
		ch, err = c.tru.newChannel(c.conn, addr)
		if err != nil {
			err = fmt.Errorf(errCantCreateChannel, err)
			// TODO: print some message if can't create ot get channel?
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

			// Packet with id more than expectedID placed to receive queue and wait
			// previouse packets
			case dist > 0:
				data := append([]byte{}, p[:n]...)
				if err :=	ch.rq.add(header.id, data); err != nil {
					// Set channel drop statistic
					ch.Stat.incDrop()
				}

			// Valid data packet received (id == expectedID)
			case dist == 0:
				if !readChannelBusy {
					// Send data packet to readChannel
					ch.newExpectedId()
					n = n - headerLen
					c.tru.readChannel <- readChannelData{
						n, addr, err, append([]byte{}, p[headerLen:]...),
					}
					ch.Stat.incRecv()
				} else {
					// Drop this data packet
					processed = false
					if !gotFromReceiveQueue {
						ch.setLastdata()
					}
				}
			}

			// Send answer
			if !gotFromReceiveQueue && processed {
				data, _ := headerPacket{header.id, pAck}.MarshalBinary()
				c.conn.WriteTo(data, addr)
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
			c.conn.WriteTo(data, addr)

		// pPong (ping answer) received
		case pPong:
		}
	}
}

// WriteTo writes a packet with payload p to addr.
// WriteTo can be made to time out and return an Error after a
// fixed time limit; see SetDeadline and SetWriteDeadline.
// On packet-oriented connections, write timeouts are rare.
func (c *truPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {

	// Get or create tru channel
	var ch *Channel
	ch, err = c.tru.newChannel(c.conn, addr)
	if err != nil {
		err = fmt.Errorf(errCantCreateChannel, err)
		return
	}

	// Get new packet id and create packet header
	id := ch.newId()
	data, err := headerPacket{id, pData}.MarshalBinary()
	if err != nil {
		return
	}
	data = append(data, p...)

	// Wait until send avalable and the same id removed from send queue
	ch.sq.writeDelay(id)

	// Write to udp
	n, err = c.conn.WriteTo(data, addr)
	n -= headerLen
	func() { ch.sq.add(id, data); ch.Stat.incSent() }()

	return
}

func (c *truPacketConn) Close() error                       { return c.conn.Close() }
func (c *truPacketConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *truPacketConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *truPacketConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *truPacketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
