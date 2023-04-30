// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"time"
)

type Channel struct {
	addr       net.Addr      // Peer address
	serverMode bool          // Server mode if true
	id         uint32        // Next send ID
	expectedID uint32        // Next expected ID
	reader     ReaderFunc    // Channels reader
	stat       statistic     // Statictic struct and receiver
	sendQueue  sendQueue     // Send queue
	recvQueue  receiveQueue  // Receive queue
	tru        *Tru          // Pointer to tru
	combine    combinePacket // Combine lage packet
	maxDataLen int           // Max data len in created packets
	*crypt                   // Crypt module
}

// const MaxUint16 = ^uint16(0)

var ErrChannelDestroyed = errors.New("channel destroyed")
var ErrChannelAlreadyDestroyed = errors.New("channel already destroyed")

// NewChannel create new tru channel by address
func (tru *Tru) newChannel(addr net.Addr, serverMode ...bool) (ch *Channel, err error) {
	tru.mu.Lock()
	defer tru.mu.Unlock()

	msg := fmt.Sprint("new channel ", addr.String())
	log.Connect.Println(msg)
	tru.statMsgs.add(msg)

	ch = &Channel{addr: addr, tru: tru, maxDataLen: tru.maxDataLen}
	if len(serverMode) > 0 {
		ch.serverMode = serverMode[0]
	}
	ch.crypt, err = tru.newCrypt()
	if err != nil {
		return
	}
	ch.sendQueue.init(ch)
	ch.recvQueue.init(ch)
	ch.stat.init(
		// Inactive
		func() {
			ch.destroy(fmt.Sprint("channel inactive, destroy ", ch.addr.String()))
		},
		// Keepalive
		func() {
			if ch.serverMode {
				return
			}
			log.Debugvvv.Println("ping", ch.addr.String())
			ch.writeToPing()
		},
	)
	// Set default send delay in client mode
	if !ch.serverMode {
		ch.stat.sendDelay = tru.sendDelay
	}
	tru.channels[addr.String()] = ch

	// Channel connect to server callback
	if ch.serverMode && ch.tru.connectcb != nil {
		ch.tru.connectcb(ch, nil)
	}

	return
}

// getChannel get tru channel by address
func (tru *Tru) getChannel(addr string) (ch *Channel, ok bool) {
	tru.mu.RLock()
	defer tru.mu.RUnlock()

	ch, ok = tru.channels[addr]
	return
}

// getChannelRandom get tru channel random
// func (tru *Tru) getChannelRandom() (ch *Channel) {
// 	tru.mu.RLock()
// 	defer tru.mu.RUnlock()

// 	if len(tru.channels) == 0 {
// 		return
// 	}

// 	keys := reflect.ValueOf(tru.channels).MapKeys()
// 	randomIndex := rand.Intn(len(keys))
// 	ch = tru.channels[keys[randomIndex].String()]

// 	return
// }

// ForEachChannel get each channel and call function f
func (tru *Tru) ForEachChannel(f func(ch *Channel)) {
	tru.mu.RLock()
	keys := reflect.ValueOf(tru.channels).MapKeys()
	tru.mu.RUnlock()
	for i := range keys {
		f(tru.channels[keys[i].String()])
	}
}

// writeToPunch write puch packet
func (tru *Tru) WriteToPunch(data []byte, addri interface{}) (addr net.Addr, err error) {
	data, err = tru.newPacket().SetStatus(statusPunch).SetData(data).MarshalBinary()
	if err != nil {
		return
	}
	return tru.WriteTo(data, addri)
}

// setReader sets channels reafer
func (ch *Channel) setReader(reader ReaderFunc) {
	ch.reader = reader
}

// destroy destroy channel
func (ch *Channel) destroy(msg string) {
	if ch == nil {
		return
	}

	// Send error event to readers
	if ch.reader != nil {
		ch.reader(ch, nil, ErrChannelDestroyed)
	}
	if ch.tru.reader != nil {
		ch.tru.reader(ch, nil, ErrChannelDestroyed)
	}

	// Destroy sendQueue and statistic
	ch.sendQueue.destroy()
	ch.stat.destroy()

	// Log messages
	log.Connect.Println(msg)
	ch.tru.statMsgs.add(msg)

	// Delete channel from channels
	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()
	delete(ch.tru.channels, ch.addr.String())
}

// Close tru channel
func (ch *Channel) Close() {
	if ch.stat.isDestroyed() {
		return
	}
	ch.writeToDisconnect()
	ch.destroy(fmt.Sprint("channel close, destroy ", ch.addr.String()))
}

// Destroyed return true if channel is already destroyed
func (ch *Channel) Destroyed() bool {
	return ch.stat.isDestroyed()
}

// Addr return tru channels address
func (ch *Channel) Addr() net.Addr {
	return ch.addr
}

// String return channels address in string
func (ch *Channel) String() string {
	return ch.Addr().String()
}

// IP return remote IP
func (ch *Channel) IP() net.IP {
	return ch.Addr().(*net.UDPAddr).IP
}

// Port return remote port
func (ch *Channel) Port() int {
	return ch.Addr().(*net.UDPAddr).Port
}

// Addr return tru channels address
func (ch *Channel) Triptime() time.Duration {
	return ch.getTripTime()
}

// ServerMode return true if there is server mode channel
func (ch *Channel) ServerMode() bool {
	return ch.serverMode
}

// WriteTo writes packet with data to tru channel. Second parameter delivery is
// callback function of PacketDeliveryFunc func, it calls when packet deliverid
// to remout peer. The third parameter is the delivery callback timeout. The
// PacketDeliveryFunc callback parameter is pac - pointer to send packet, and
// err - timeout error or success if nil.
func (ch *Channel) WriteTo(data []byte, delivery ...interface{}) (id int, err error) {
	return ch.splitPacket(data, func(data []byte, split int) (int, error) {
		return ch.writeTo(data, statusData|split, delivery)
	})
}

// writeTo writes a packet with status and data to channel
func (ch *Channel) writeTo(data []byte, stat int, delivery []interface{}, ids ...int) (id int, err error) {
	if ch.stat.isDestroyed() {
		err = ErrChannelDestroyed
		return
	}

	// Parse delivery parameter
	var deliveryFunc PacketDeliveryFunc
	var deliveryTimeout time.Duration = DeliveryTimeout
	for _, i := range delivery {
		switch v := i.(type) {
		case PacketDeliveryFunc:
			deliveryFunc = v
		case func(*Packet, error):
			deliveryFunc = v
		case time.Duration:
			deliveryTimeout = v
		default:
			err = errors.New("writeTo got wrong type of delivery parameter")
			return
		}
	}

	status := stat &^ statusSplit

	// Calculate and execute delay for client mode data packets
	ch.writeToDelay(status)

	// Set packet id and encript data
	if len(ids) > 0 {
		id = ids[0]
	}
	if status == statusData {
		id = ch.newID()
		data, err = ch.encryptPacketData(id, data)
		if err != nil {
			return
		}
	}

	// Create packet
	pac := ch.tru.newPacket().SetID(id).SetStatus(stat).SetData(data)

	// Add data packet to send queue and Set packet retransmit time
	if status == statusData {
		ch.setRetransmitTime(pac)
		ch.sendQueue.add(pac)
		if stat == statusData {
			pac.SetDeliveryTimeout(deliveryTimeout)
			pac.SetDelivery(deliveryFunc)
			ch.stat.setSend()
		}
		ch.stat.setLastSend(time.Now())
	}

	// Send disconnect packet immediately
	if status == statusDisconnect {
		data, _ := pac.MarshalBinary()
		_, err = ch.tru.WriteTo(data, ch.addr)
		return
	}

	// Drop packet for testing if drop flag is set. Drops every value of
	// drop packet. If drop contain 5 than every 5th packet will be dropped
	if *drop > 0 && status == statusData && !ch.serverMode && rand.Intn(*drop) == 0 {
		return
	}

	// Send to write channel
	ch.writeToSender(pac)

	return
}

// writeToDelay calculate and execute delay for client mode data packets
func (ch *Channel) writeToDelay(status int) {

	// Use client mode data packages only
	if !(!ch.serverMode && status == statusData) {
		return
	}

	// Sleep up to 300 microseconds while fist packet has retransmit attempt
	var retransmitDelayCount = 0
	const minSendDelay = 15
	for rta := ch.sendQueue.getRetransmitAttempts(); rta > 0 && retransmitDelayCount < 20; retransmitDelayCount++ {
		time.Sleep(minSendDelay * time.Microsecond)
		rta = ch.sendQueue.getRetransmitAttempts()
	}

	// Get current delay
	var chSendDelay = ch.stat.getSendDelay()
	delay := time.Duration(chSendDelay) * time.Microsecond
	if time.Since(ch.stat.lastDelayCheck) > 30*time.Millisecond {

		// Claculate new delay
		if retransmitDelayCount == 0 {
			switch {
			case ch.stat.sendDelay > 100:
				chSendDelay -= 10
			case ch.stat.sendDelay > minSendDelay:
				chSendDelay -= 1
			}
		} else {
			chSendDelay += 10
		}

		// Set new delay
		ch.stat.lastDelayCheck = time.Now()
		ch.stat.setSendDelay(chSendDelay)
	}

	// Execute current delay
	if since := time.Since(ch.stat.getLastSend()); since < delay {
		time.Sleep(delay - since)
	}
}

// writeToPing writes ping packet to channel
func (ch *Channel) writeToPing() (err error) {
	_, err = ch.writeTo(nil, statusPing, nil)
	return
}

// writeToPong writes pong packet to channel
func (ch *Channel) writeToPong() (err error) {
	_, err = ch.writeTo(nil, statusPong, nil)
	return
}

// writeToAck writes ack packet to channel
func (ch *Channel) writeToAck(pac *Packet) (err error) {
	_, err = ch.writeTo(nil, statusAck, nil, pac.ID())
	return
}

// writeToDisconnect write disconnect packet
func (ch *Channel) writeToDisconnect() (err error) {
	_, err = ch.writeTo(nil, statusDisconnect, nil)
	return
}

// writeToSender write packet to sender process channel
func (ch *Channel) writeToSender(pac *Packet) {
	ch.tru.senderCh <- senderChData{ch, pac}
}

// newID create new channels packet id
func (ch *Channel) newID() (id int) {
	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()

	id = int(ch.id)

	ch.id++
	if ch.id >= packetIDLimit {
		ch.id = 0
	}

	return
}

// newExpectedID create new channels packet expected id
func (ch *Channel) newExpectedID() (id int) {
	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()

	ch.expectedID++
	if ch.expectedID >= packetIDLimit {
		ch.expectedID = 0
	}

	id = int(ch.expectedID)

	return
}

// setTripTime calculate return and set trip time to statistic
func (ch *Channel) setTripTime(id int) (tt time.Duration, err error) {
	_, pac, ok := ch.sendQueue.get(id)
	if !ok {
		err = errors.New("packet not found")
		return
	}

	ch.stat.Lock()
	defer ch.stat.Unlock()

	tt = time.Since(pac.time)
	ch.stat.tripTime = tt
	if ch.stat.tripTimeMidle != 0 {
		ch.stat.tripTimeMidle = (ch.stat.tripTimeMidle*9 + tt) / 10
	} else {
		ch.stat.tripTimeMidle = tt
	}

	return
}

// getTripTime return current channel trip time
func (ch *Channel) getTripTime() time.Duration {
	ch.stat.RLock()
	defer ch.stat.RUnlock()

	return ch.stat.tripTimeMidle
}

// setRetransmitTime set retransmit time to packet
func (ch *Channel) setRetransmitTime(pac *Packet) (rt time.Time, err error) {
	rtt := minRTT
	if tt := ch.getTripTime(); tt == 0 {
		rtt = startRTT
	} else {
		rtt += tt
	}

	if ra := pac.getRetransmitAttempts(); ra > 0 {
		rtt *= time.Duration(ra + 1)
	}

	if rtt > maxRTT {
		rtt = maxRTT
	}

	pac.setRetransmitTime(rtt)

	return
}
