// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU process standard golang net.Listener and net.Conn interfaces

package tru

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Trunet is receiver to create Tru net.Listener interface
type Trunet struct {
	*Tru
	accept acceptChannel
}
type acceptChannel chan *Channel

// Common tru object
var truCommon *Tru

// Listener is a tru generic network listener for stream-oriented protocols.
func Listen(network, address string) (listener net.Listener, err error) {
	listener, err = newTrunet(address, true)
	return
}

// Dial connects to the address on the tru network
func Dial(network, address string) (conn net.Conn, err error) {

	trunet, err := newTrunet(":0", true)
	if err != nil {
		return
	}

	ch, err := trunet.Connect(address)
	if err != nil {
		return
	}
	c := trunet.newConn(ch)
	ch.setReader(c.reader)
	conn = c
	return
}

// newTrunet creates new Trunet object
func newTrunet(address string, useTruCommon bool) (trunet *Trunet, err error) {
	trunet = new(Trunet)

	// Parse the address and get port value
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		err = fmt.Errorf("wrong address: %s", address)
		return
	}
	port, err := strconv.Atoi(addr[1])
	if err != nil {
		err = fmt.Errorf("wrong port %s in address: %s", addr[1], address)
		return
	}

	// Make accept golan channel
	trunet.accept = make(acceptChannel)

	// Create new tru object or use existing
	var truObj *Tru = truCommon
	if !useTruCommon || truObj == nil {
		truObj, err = New(port, trunet.connected, logLevel, Stat(showStats))
		if err != nil {
			err = fmt.Errorf("can't create new tru object, error: %s", err)
			return
		}
		if useTruCommon {
			truCommon = truObj
		}
	}
	trunet.Tru = truObj

	return
}

// reader reads packets from connected peers
func (conn *Conn) reader(ch *Channel, pac *Packet, err error) (processed bool) {
	if conn.ch == nil {
		return
	}

	// Check channel destroyed and close Conn (connection)
	if err != nil {
		if err == ErrChannelDestroyed {
			err = conn.Close()
			if err != nil {
				// if connection already closed
				return
			}
			// tru channel destroyed, connection closed
			return
		}
		// Some other errors (I think it never happens)
		log.Error.Println("got error in reader:", err)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			// This error happens if users application don't read data messages
			// sent to it, when connection closed.
			log.Debugvv.Println("send on closed channel panic occurred:", err)
		}
	}()

	conn.read.ch <- pac.Data()

	return
}

// connected to tru callback function
func (t Trunet) connected(ch *Channel, err error) {
	if err != nil {
		return
	}
	t.accept <- ch
}

// Create new Conn object
func (t Trunet) newConn(ch *Channel) *Conn {
	return &Conn{ch, t.LocalAddr(), ch.Addr(), connRead{ch: make(connReadChan)}}
}

// Accept waits for and returns the next connection to the listener.
func (t Trunet) Accept() (conn net.Conn, err error) {
	ch := <-t.accept
	c := t.newConn(ch)
	ch.setReader(c.reader)
	conn = c
	return
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (t Trunet) Close() (err error) {
	t.Tru.Close()
	return
}

// Addr returns the listener's network address.
func (t Trunet) Addr() (addr net.Addr) { return }

// Conn is a generic stream-oriented network connection.
//
// Multiple goroutines may invoke methods on a Conn simultaneously.
type Conn struct {
	ch         *Channel
	localAddr  net.Addr
	remoteAddr net.Addr
	read       connRead
}
type connRead struct {
	ch   connReadChan // Read channel receive data form tru channel reader
	buf  []byte       // Read data which was not send in previuse call
	cont bool         // Need send os.EOF
}
type connReadChan chan connReadChanData
type connReadChanData []byte

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *Conn) Read(b []byte) (n int, err error) {

	// Return io.EOF
	if len(c.read.buf) == 0 && c.read.cont {
		c.read.cont = false
		err = io.EOF
		return
	}

	// Get data from reader
	if len(c.read.buf) == 0 {
		chanData, ok := <-c.read.ch
		if !ok {
			// Read channel closed
			err = ErrChannelDestroyed
			return
		}
		c.read.buf = chanData
		c.read.cont = true
	}

	// Copy data to input slice
	n = copy(b, c.read.buf)
	c.read.buf = c.read.buf[n:]

	return
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c Conn) Write(b []byte) (n int, err error) {
	if c.ch == nil {
		err = ErrChannelDestroyed
		return
	}
	_, err = c.ch.WriteTo(b)
	if err == nil {
		n = len(b)
	}
	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *Conn) Close() (err error) {

	if c.ch == nil {
		err = ErrChannelAlreadyDestroyed
		return
	}

	ch := c.ch
	c.ch = nil
	ch.Close()
	close(c.read.ch)

	return
}

// LocalAddr returns the local network address, if known.
func (c Conn) LocalAddr() net.Addr { return c.localAddr }

// RemoteAddr returns the remote network address, if known.
func (c Conn) RemoteAddr() net.Addr { return c.remoteAddr }

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c Conn) SetDeadline(t time.Time) (err error) { return }

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c Conn) SetReadDeadline(t time.Time) (err error) { return }

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c Conn) SetWriteDeadline(t time.Time) (err error) { return }
