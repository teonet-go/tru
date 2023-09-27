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

	// Basic constants

	readChannelLen = 0 // 4 * 1024
	readBufLen     = 2 * 1024
	numReaders     = 4

	// Tuning constants

	// Check send queue retransmits every sqCheckAfter.
	sqCheckAfter = 10 * time.Millisecond

	// Send queue wait ack to data packets extra time (triptime + extraTime).
	sqWaitPacketExtraTime = 0 * time.Millisecond

	// If packet retransmits happend then: Calculate trip time from first send
	// of this packet if true or from last retransmit if false.
	ttFromFirstSend = true

	// When claculate triptime uses ttCalcMiddle last packets to get moddle
	// triptime.
	ttCalcMiddle = 10
)

// Tru is main tru data structure and methods reciever
type Tru struct {
	channels      // Channels map
	*sync.RWMutex // Channels map mutex
	readChannel   chan readChannelData

	started time.Time // Tru started time
}
type readChannelData struct {
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
	truConn := &truPacketConn{conn: conn, tru: tru, Mutex: new(sync.Mutex)}
	//for i := 0; i < numReaders; i++ {
	go truConn.readFrom()
	//}
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

	// Check channel exists and return existing channel
	if ch, ok := tru.channels[addrStr]; ok {
		return ch, nil
	}

	// Create new channel and add it to channels map
	ch, err = newChannel(tru, conn, addrStr, func() { tru.delChannel(ch) })
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

	ch.setClosed()
	close(ch.processChan)
	delete(tru.channels, addr)

	return nil
}

// type channels chan channelsData
type channelsData struct {
	addr string
	ch   *Channel
}

// delChannel removes selected Tru channel or return error if does not exist
func (tru *Tru) listChannels() chan channelsData {
	chs := make(chan channelsData)
	go func() {
		tru.RLock()
		defer tru.RUnlock()
		for addr, ch := range tru.channels {
			chs <- channelsData{addr, ch}
		}
		close(chs)
	}()
	return chs
}

// truPacketConn is a generic packet-oriented network connection.
//
// Multiple goroutines may invoke methods on a PacketConn simultaneously.
type truPacketConn struct {
	conn net.PacketConn
	tru  *Tru
	*sync.Mutex
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
	addr, err = r.addr, r.err
	n = copy(p, r.data)
	return
}

// readFrom is reader worker
func (c *truPacketConn) readFrom() (err error) {

	var p = make([]byte, readBufLen)

	for {
		// Read data from connection
		n, addr, err := c.conn.ReadFrom(p)
		if err != nil {
			log.Println("read from error:", err)
			return err
		}

		// Get or create channel
		var ch *Channel
		ch, err = c.tru.newChannel(c.conn, addr)
		if err != nil {
			err = fmt.Errorf(errCantCreateChannel, err)
			// TODO: print some message if can't create ot get channel?
			continue
		}
		if ch.closed() {
			continue
		}

		// Send received data to process channel (safe to channel closed)
		ch.processChan <- append([]byte{}, p[:n]...)
	}

	// return
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
	ch.sq.writeDelay(ch, id)

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
