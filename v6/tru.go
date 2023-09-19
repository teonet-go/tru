package tru

import (
	"fmt"
	"net"
	"slices"
	"sync"
	"time"
)

type Tru struct {
	chm channels
	*sync.RWMutex
}
type channels map[string]*Channel

// New creates new Tru object
func New() *Tru {
	tru := new(Tru)
	tru.chm = make(channels)
	tru.RWMutex = new(sync.RWMutex)
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
	return truConn, err
}

// GetChannel gets Tru channel by addr
func (tru *Tru) GetChannel(addr net.Addr) *Channel {
	tru.RLock()
	defer tru.RUnlock()

	ch, ok := tru.chm[addr.String()]
	if !ok {
		return nil
	}

	return ch
}

// newChannel creates new Tru channel with addr or return existing
func (tru *Tru) newChannel(conn net.PacketConn, addr net.Addr) *Channel {
	tru.Lock()
	defer tru.Unlock()

	addrStr := addr.String()

	if ch, ok := tru.chm[addrStr]; ok {
		return ch
	}

	ch, err := newChannel(conn, addrStr)
	if err != nil {
		return nil
	}

	tru.chm[addrStr] = ch

	return ch
}

// getFromReceiveQueue gets saved packet with expected id from receive queue on
// any channel
func (tru *Tru) getFromReceiveQueue(p []byte) (n int, addr net.Addr, err error) {
	tru.RLock()
	defer tru.RUnlock()

	for _, ch := range tru.chm {
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

	for {
		// Get saved packet with expected id from receive queue on any channel
		var gotFromReceiveQueue bool
		n, addr, err = c.tru.getFromReceiveQueue(p)
		if err == nil {
			gotFromReceiveQueue = true
		}

		// Read data from connection
		if !gotFromReceiveQueue {
			n, addr, err = c.conn.ReadFrom(p)
			if err != nil {
				return
			}
		}

		// Unmarshal header
		header := headerPacket{}
		err = header.UnmarshalBinary(p)
		if err != nil {
			return
		}

		// Get or create channel
		ch := c.tru.GetChannel(addr)
		if ch == nil {
			ch = c.tru.newChannel(c.conn, addr)
		}

		// Send tru answer or process answer depend of header type
		switch header.ptype {

		// Data received
		case pData:
			// Send answer
			if !gotFromReceiveQueue {
				data, _ := headerPacket{header.id, pAnswer}.MarshalBinary()
				c.conn.WriteTo(data, addr)
			}

			// Check expected id distance
			dist := ch.distance(header.id)
			switch {

			// Already processed packet (id < expectedID)
			case dist < 0:
				// TODO: Set channel drop statistic
				// ch.stat.setDrop()

				// Continue reading
				// return c.ReadFrom(p)

			// Packet with id more than expectedID placed to receive queue and wait
			// previouse packets
			case dist > 0:
				_, ok := ch.rq.get(header.id)
				if !ok {
					data := slices.Clone(p[:n])
					ch.rq.add(header.id, data)
				} else {
					// TODO: Set channel drop statistic
					// ch.stat.setDrop()
				}
				// Continue reading
				// return c.ReadFrom(p)

			// Valid data packet received (id == expectedID)
			case dist == 0:
				// Prepare data to return
				copy(p, p[headerLen:])
				n = n - headerLen
				ch.newExpectedId()
				return
			}

		// Answer received
		case pAnswer:
			// Save answer statistic, calculate triptime and remove package from
			// send queue
			if ch != nil {
				ch.incAnswer()
				if pac, ok := ch.sq.del(header.id); ok {
					ch.calcTriptime(pac)
				}
			}
			// Continue reading
			// return c.ReadFrom(p)

		// Ping received
		case pPing:

		// pPong (ping answer) received
		case pPong:
		}
	}
}

func (c *truPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {

	ch := c.tru.GetChannel(addr)
	if ch == nil {
		ch = c.tru.newChannel(c.conn, addr)
		if ch == nil {
			err = fmt.Errorf("can't create new tru channel: %v", addr)
			return
		}
		// fmt.Println("new send channel", ch.addr)
	}

	id := ch.newId()
	header := headerPacket{id, 0}
	data, _ := header.MarshalBinary()
	data = append(data, p...)

	ch.sq.writeDelay()

	n, err = c.conn.WriteTo(data, addr)
	n -= headerLen

	go ch.sq.add(id, data)

	return
}

func (c *truPacketConn) Close() error {
	return c.conn.Close()
}

func (c *truPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *truPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *truPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *truPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
