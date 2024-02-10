package tru

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

const errCantCreateChannel = "can't create tru channel: %s"

const (

	// Basic constants

	readChannelLen = 1 * 1024
	readBufLen     = 2 * 1024
	numReaders     = 4

	// Tuning constants

	// Check send queue retransmits every sqCheckAfter.
	sqCheckAfter = 30 * time.Millisecond

	// Send queue wait ack to data packets extra time (triptime + extraTime).
	sqWaitPacketExtraTime = 250 * time.Millisecond // 250ms

	// If packet retransmits happend then: Calculate trip time from first send
	// of this packet if true or from last retransmit if false.
	ttFromFirstSend = true

	// When claculate triptime uses ttCalcMiddle last packets to get middle
	// triptime.
	ttCalcMiddle = 10
)

var ErrChannelDoesNotExist = fmt.Errorf("channel does not exist")

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
	if err != nil {
		return nil, err
	}
	truConn := &PacketConn{conn: conn, tru: tru, Mutex: new(sync.Mutex)}
	for i := 0; i < numReaders; i++ {
		go truConn.readFrom()
	}
	return truConn, err
}

// GetChannel gets Tru channel by addr.
func (tru *Tru) GetChannel(addr net.Addr) (*Channel, error) {
	tru.RLock()
	defer tru.RUnlock()

	ch, ok := tru.channels[addr.String()]
	if !ok {
		return nil, ErrChannelDoesNotExist
	}

	return ch, nil
}

// getChannel creates new Tru channel with addr or return existing.
// It takes udp connection and addr as arguments and returns created or existing
// channel and error.
func (tru *Tru) getChannel(conn net.PacketConn, addr net.Addr) (ch *Channel,
	err error) {

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

// delChannel removes selected Tru channel or return error if does not exist.
func (tru *Tru) delChannel(ch *Channel) error {
	tru.Lock()
	defer tru.Unlock()

	addr := ch.addr.String()
	if _, ok := tru.channels[addr]; !ok {
		return ErrChannelDoesNotExist
	}
	delete(tru.channels, addr)

	ch.setClosed()
	close(ch.processChan)

	return nil
}

// type channels chan channelsData.
type channelsData struct {
	addr string
	ch   *Channel
}

// listChannels returns Tru channelsData channel which contains all Tru channels.
func (tru *Tru) listChannels() chan channelsData {
	tru.RLock()
	defer tru.RUnlock()

	// Get channels slice
	chs := make(chan channelsData)
	var chanels []channelsData
	for addr, ch := range tru.channels {
		chanels = append(chanels, channelsData{addr, ch})
	}

	// Sort channels slice
	sort.Slice(chanels, func(i, j int) bool {
		return chanels[i].addr < chanels[j].addr
	})

	// Send channels to output channel
	go func() {
		for _, c := range chanels {
			chs <- channelsData{c.addr, c.ch}
		}
		close(chs)
	}()

	return chs
}
