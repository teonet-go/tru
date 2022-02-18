// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channels module

package tru

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

type Channel struct {
	addr       net.Addr      // Peer address
	serverMode bool          // Server mode if true
	id         uint16        // Next send ID
	expectedID uint16        // Next expected ID
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

// NewChannel create new tru channel by address
func (tru *Tru) newChannel(addr net.Addr, serverMode ...bool) (ch *Channel, err error) {
	tru.mu.Lock()
	defer tru.mu.Unlock()

	msg := fmt.Sprint("new channel ", addr.String())
	log.Println(msg)
	tru.addToMsgsLog(msg)

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
			log.Println("channel ping", ch.addr.String())
			ch.writeToPing()
		},
	)
	ch.stat.sendDelay = tru.sendDelay
	tru.cannels[addr.String()] = ch
	return
}

// getChannel get tru channel by address
func (tru *Tru) getChannel(addr string) (ch *Channel, ok bool) {
	tru.mu.RLock()
	defer tru.mu.RUnlock()

	ch, ok = tru.cannels[addr]
	return
}

// addToMsgsLog add message to log
func (tru *Tru) addToMsgsLog(msg string) {
	const layout = "2006-01-02 15:04:05.000000"
	msg = fmt.Sprintf("%v %s", time.Now().Format(layout), msg)
	tru.statLogMsgs = append(tru.statLogMsgs, msg)
}

// destroy destroy channel
func (ch *Channel) destroy(msg string) {
	if ch == nil {
		return
	}

	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()

	log.Println("channel destroy", ch.addr.String())

	ch.sendQueue.destroy()
	ch.stat.destroy()

	delete(ch.tru.cannels, ch.addr.String())
	ch.tru.addToMsgsLog(msg)
}

// Close tru channel
func (ch *Channel) Close() {
	ch.writeToDisconnect()
	// time.Sleep(minRTT)
	ch.destroy(fmt.Sprint("channel close, destroy ", ch.addr.String()))
}

// Addr return tru channels address
func (ch *Channel) Addr() net.Addr {
	return ch.addr
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
		err = errors.New("channel destroyed")
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
			err = errors.New("got wrong type delivery parameter")
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
		ch.stat.lastSend = time.Now()
	}

	// Send disconnect immediately
	if status == statusDisconnect {
		data, _ := pac.MarshalBinary()
		ch.tru.writeTo(data, ch.addr)
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
	if !(!ch.serverMode && status == statusData) {
		return
	}

	// Wait up to 100 ms if fist packet has retransmit attempt
	var retransmitDelayCount = 0
	for rta := ch.sendQueue.getRetransmitAttempts(); rta > 0 && retransmitDelayCount < 10; retransmitDelayCount++ {
		time.Sleep(10000 * time.Microsecond) // 10 ms sleet if retransmit attempt set now
	}

	// Get current delay
	delay := time.Duration(ch.stat.sendDelay) * time.Microsecond

	// Claculate new delay
	var chSendDelay = ch.stat.getSendDelay()
	if retransmitDelayCount == 0 {
		switch {
		case ch.stat.sendDelay > 100:
			chSendDelay -= 10
		case ch.stat.sendDelay > 30:
			chSendDelay -= 1
		}
	} else {
		chSendDelay += 10
	}

	// Set new delay
	if time.Since(ch.stat.lastDelayCheck) > 50*time.Millisecond {
		ch.stat.lastDelayCheck = time.Now()
		ch.stat.setSendDelay(chSendDelay)
	}

	// Execute current delay
	if since := time.Since(ch.stat.lastSend); since < delay {
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

// writeToSender write packet to sender proccess channel
func (ch *Channel) writeToSender(pac *Packet) {
	ch.tru.senderCh <- senderChData{ch, pac}
}

// newID create new channels packet id
func (ch *Channel) newID() (id int) {
	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()

	id = int(ch.id)
	ch.id++

	return
}

// newExpectedID create new channels packet expected id
func (ch *Channel) newExpectedID() (id int) {
	ch.tru.mu.Lock()
	defer ch.tru.mu.Unlock()

	ch.expectedID++
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
	ch.stat.tripTimeMidle = (ch.stat.tripTimeMidle*9 + tt) / 10

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

	if pac.retransmitAttempts > 0 {
		rtt *= time.Duration(pac.retransmitAttempts + 1)
	}

	if rtt > maxRTT {
		rtt = maxRTT
	}

	pac.retransmitTime = time.Now().Add(rtt)
	pac.time = time.Now()

	return
}
