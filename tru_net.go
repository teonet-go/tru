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

// TruNet is receiver to create Tru net.Listener interface
type TruNet struct {
	*Tru
	accept acceptChannel
}
type acceptChannel chan *Channel

// Listener is a tru generic network listener for stream-oriented protocols.
func Listen(network, address string) (listener net.Listener, err error) {
	trunet := new(TruNet)

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

	trunet.accept = make(acceptChannel)
	trunet.Tru, err = New(port, trunet.connected, "Info", Stat(true))

	listener = trunet
	return
}

// func newTru() (t *Tru, err error) {
// 	// tru, err := tru.New(*port, Reader, tru.Stat(*stat),
// 	// 	tru.Hotkey(*hotkey), log, *loglevel, teolog.Logfilter(*logfilter))
// 	t, err = New(7070)
// 	if err != nil {
// 		log.Error.Fatal("can't create tru, err: ", err)
// 	}
// 	// defer tru.Close()
// 	// tru.SetMaxDataLen(*datalen)
// }

// reader read packets from connected peers
func (conn Conn) reader(ch *Channel, pac *Packet, err error) (processed bool) {
	if err != nil {
		if err == ErrChannelDestroyed {
			log.Info.Print("channel destroyed")
			conn.read.ch <- connReadChanData{nil, err}
			return
		}
		log.Info.Println("got error in reader:", err)
		return
	}
	// log.Info.Printf("got %d byte from %s, id %d, data len: %d\n",
	// 	pac.Len(), ch.Addr().String(), pac.ID(), len(pac.Data()))

	conn.read.ch <- connReadChanData{pac.Data(), nil}

	return
}

// connected to tru callback function
func (t TruNet) connected(ch *Channel, err error) {
	if err != nil {
		return
	}
	// log.Info.Println("channel connected", ch, err)
	t.accept <- ch
}

// Accept waits for and returns the next connection to the listener.
func (t TruNet) Accept() (c net.Conn, err error) {
	ch := <-t.accept
	// log.Info.Println("channel connected", ch, err)
	conn := &Conn{ch, t.LocalAddr(), connRead{ch: make(connReadChan)}}
	ch.setReader(conn.reader)
	c = conn
	return
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (t TruNet) Close() (err error) { return }

// Addr returns the listener's network address.
func (t TruNet) Addr() (addr net.Addr) { return }

// Conn is a generic stream-oriented network connection.
//
// Multiple goroutines may invoke methods on a Conn simultaneously.
type Conn struct {
	ch        *Channel
	localAddr net.Addr
	read      connRead
}
type connRead struct {
	ch   connReadChan // Read channel receive data form tru channel reader
	buf  []byte       // Read data wich was not send in previuse call
	cont bool         // Need send os.EOF
}
type connReadChan chan connReadChanData
type connReadChanData struct {
	data []byte
	err  error
}

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
			err = ErrChannelDestroyed // channed destroed and connection closed
			return
		}
		if chanData.err != nil {
			err = chanData.err
			return
		}
		c.read.buf = chanData.data
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
	_, err = c.ch.WriteTo(b)
	if err == nil {
		n = len(b)
	}
	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c Conn) Close() (err error) {
	c.ch.Close()
	close(c.read.ch)
	return
}

// LocalAddr returns the local network address, if known.
func (c Conn) LocalAddr() net.Addr { return c.localAddr }

// RemoteAddr returns the remote network address, if known.
func (c Conn) RemoteAddr() net.Addr { return &Addr{c.ch} }

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

// Addr represents a network end point address.
//
// The two methods Network and String conventionally return strings
// that can be passed as the arguments to Dial, but the exact form
// and meaning of the strings is up to the implementation.
type Addr struct {
	ch *Channel
}

func (a Addr) Network() string { return "udp" }         // name of the network (for example, "tcp", "udp")
func (a Addr) String() string  { return a.ch.String() } // string form of address (for example, "192.0.2.1:25", "[2001:db8::1]:80")
