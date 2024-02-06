package tru

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// PacketConn is a generic tru network connection.
//
// Multiple goroutines may invoke methods on a PacketConn simultaneously.
type PacketConn struct {
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
func (c *PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	// Read processed message from read go channel
	r := <-c.tru.readChannel
	addr, err = r.addr, r.err
	n = copy(p, r.data)
	return
}

// readFrom is reader worker
func (c *PacketConn) readFrom() (err error) {

	log.Printf("reader started, addr: %s\n", c.conn.LocalAddr().String())
	defer log.Printf("reader stopped, addr: %s\n", c.conn.LocalAddr().String())

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

		// Send received data to process channel (safe send on closed channel)
		if ch.closed() || safeSend(ch.processChan, append([]byte{}, p[:n]...)) {
			log.Println("send on closed or busy channel, channel length:",
				len(ch.processChan))
			continue
		}

		// ch.processChan <- append([]byte{}, p[:n]...)
	}
}

// safeSend sends value to channel if it is not closed.
func safeSend[T any](ch chan T, value T) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()
	ch <- value
	return false
}

// WriteTo writes a packet with payload p to addr.
// WriteTo can be made to time out and return an Error after a
// fixed time limit; see SetDeadline and SetWriteDeadline.
// On packet-oriented connections, write timeouts are rare.
func (c *PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {

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

// WriteToNoWait sends a packet with payload p to addr without waiting. 
// Use it in server responses to client requests.
func WriteToNoWait(conn net.PacketConn, p []byte, addr net.Addr, f ...func(n int, err error)) {
	go func() {
		n, err := conn.WriteTo(p, addr)
		if len(f) > 0 {
			f[0](n, err)
		}
	}()
}

func (c *PacketConn) Close() error                       { return c.conn.Close() }
func (c *PacketConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *PacketConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *PacketConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *PacketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
