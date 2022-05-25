// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channel statistic module

package tru

import (
	"flag"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kirill-scherba/stable"
	"github.com/teonet-go/tru/term"
)

type statistic struct {
	destroyed          bool        // Channel is destoroyed
	started            time.Time   // Channel started time
	lastActivity       time.Time   // Last activity in channel (last received)
	lastSend           time.Time   // Last send to remote peer
	lastDelayCheck     time.Time   // Last delay check
	checkActivityTimer *time.Timer // Check activity timer
	sendDelay          int         // Client send delay

	tripTime      time.Duration
	tripTimeMidle time.Duration
	send          int64 // Number of send packets
	sendSpeed     speed // Send speed in packets/sec
	ackRecv       int64 // Number of ack to send received
	ackRecvDrop   int64 // Number of dropped ack to send received
	retransmit    int64 // Number of retransmit send packets
	recv          int64 // Number of received packets
	recvSpeed     speed // Receive speed in packets/sec
	drop          int64 // Number of droped received packets, duplicate packets

	sync.RWMutex
}

const (
	checkInactiveAfter      = 500 * time.Millisecond
	pingInactiveAfter       = 4 * time.Second
	disconnectInactiveAfter = 6 * time.Second
)

var stathide = flag.Bool("stathide", false, "hide statistic (for debuging)")

// init statistic
func (s *statistic) init(inactive, keepalive func()) {
	s.started = time.Now()
	s.setLastActivity()
	s.checkActivity(inactive, keepalive)
	s.sendSpeed.init()
	s.recvSpeed.init()
}

// destroy statistic
func (s *statistic) destroy() {
	s.Lock()
	defer s.Unlock()

	s.checkActivityTimer.Stop()
	s.sendSpeed.destroy()
	s.recvSpeed.destroy()
	s.destroyed = true
}

// isDestroyed return true if channel is already destroyed
func (s *statistic) isDestroyed() bool {
	s.RLock()
	defer s.RUnlock()
	return s.destroyed
}

// setSend set one packet send
func (s *statistic) setSend() {
	s.Lock()
	defer s.Unlock()

	s.send++
	s.sendSpeed.add()
}

// setAckReceived set one ack packet received
func (s *statistic) setAckReceived() {
	s.Lock()
	defer s.Unlock()

	s.ackRecv++
}

// setAckDropReceived set one ack dropped packet received
func (s *statistic) setAckDropReceived() {
	s.Lock()
	defer s.Unlock()

	s.ackRecvDrop++
}

// setRecv set one packet received
func (s *statistic) setRecv() {
	s.Lock()
	defer s.Unlock()

	s.recv++
	s.recvSpeed.add()
}

// setLastActivity set channels last activity time
func (s *statistic) setLastActivity() {
	s.Lock()
	defer s.Unlock()

	s.lastActivity = time.Now()
}

// getLastActivity return channels last activity time
func (s *statistic) getLastActivity() time.Time {
	s.RLock()
	defer s.RUnlock()

	return s.lastActivity
}

// setRetransmit set channels retransmit packet
func (s *statistic) setRetransmit() {
	s.Lock()
	defer s.Unlock()

	s.retransmit++
}

// setDrop set channels drop packet
func (s *statistic) setDrop() {
	s.Lock()
	defer s.Unlock()

	s.drop++
}

// setSendDelay set channels send delay
func (s *statistic) setSendDelay(sendDelay int) {
	s.Lock()
	defer s.Unlock()

	s.sendDelay = sendDelay
}

// getSendDelay return channels send delay
func (s *statistic) getSendDelay() int {
	s.RLock()
	defer s.RUnlock()

	return s.sendDelay
}

// setLastSend set channels last send time
func (s *statistic) setLastSend(t time.Time) {
	s.Lock()
	defer s.Unlock()

	s.lastSend = t
}

// getLastSend get channels last send time
func (s *statistic) getLastSend() time.Time {
	s.RLock()
	defer s.RUnlock()

	return s.lastSend
}

// checkActivity check channel activity every second and call inactive func
// if channel inactive time grate than disconnectInactiveAfter time constant,
// and call keepalive func if channel inactive time grate than pingInactiveAfter
func (s *statistic) checkActivity(inactive, keepalive func()) {
	s.Lock()
	defer s.Unlock()

	s.checkActivityTimer = time.AfterFunc(checkInactiveAfter, func() {

		lastActivity := s.getLastActivity()
		switch {

		case time.Since(lastActivity) > disconnectInactiveAfter:
			inactive()
			return

		case time.Since(lastActivity) > pingInactiveAfter:
			keepalive()
		}

		s.checkActivity(inactive, keepalive)
	})
}

// ChannelStatistic tru channel statistic data structure
type ChannelStatistic struct {
	Addr  string  // peer address
	Send  int64   // send packets
	Ssec  int64   // send per second
	Rsnd  int64   // resend packets
	Ack   int64   // ack packet received
	AckD  int64   // ack packet  received and droped (duplicate ack)
	Recv  int64   // receive packets
	Rsec  int64   // receive per second
	Drop  int64   // drop received packets
	SQ    uint    // send queue length
	RQ    uint    // receive queue length
	RTA   int     // first packet retransmit attempt
	Delay int     // client send delay
	TT    float64 // trip time
}

type ChannelsStatistic []ChannelStatistic

// Statistic get statistic
func (tru *Tru) Statistic() (stat ChannelsStatistic) {

	var wg sync.WaitGroup
	var mu sync.Mutex

	// The Statistic function needs values from send queue and send queue needs
	// values from statistic so they race. We use WaitGroup and goroutines which
	// wait while send queue unlock and add values to ChannelsStatistic slice.
	getRetransmitAttempts := func(stat ChannelsStatistic, ch *Channel, i int) {
		wg.Add(1)
		go func() {
			mu.Lock()
			defer mu.Unlock()
			defer wg.Done()
			// Add RTA and SQ to channel statistic slice
			stat[i].RTA = ch.sendQueue.getRetransmitAttempts()
			stat[i].SQ = uint(ch.sendQueue.len())
		}()
	}

	// Append channels statistic to slice
	var i int
	mu.Lock()
	tru.mu.RLock()
	for _, ch := range tru.channels {
		ch.stat.RLock()
		stat = append(stat, ChannelStatistic{
			Addr: ch.addr.String(),
			Send: ch.stat.send,
			Ssec: int64(ch.stat.sendSpeed.get()),
			Rsnd: ch.stat.retransmit,
			Ack:  ch.stat.ackRecv,
			AckD: ch.stat.ackRecvDrop,
			Recv: ch.stat.recv,
			Rsec: int64(ch.stat.recvSpeed.get()),
			Drop: ch.stat.drop,
			// SQ:  get in getRetransmitAttempts()
			RQ: uint(ch.recvQueue.len()),
			// RTA: get in getRetransmitAttempts()
			Delay: ch.stat.sendDelay,
			TT:    float64(ch.stat.tripTimeMidle.Microseconds()) / 1000.0,
		})
		ch.stat.RUnlock()
		getRetransmitAttempts(stat, ch, i)
		i++
	}
	tru.mu.RUnlock()
	mu.Unlock()
	wg.Wait()

	// Sort slice with channels statistic by address
	sort.Slice(stat, func(i, j int) bool {
		return stat[i].Addr < stat[j].Addr
	})

	return
}

// String stringlify channels statistic
func (cs *ChannelsStatistic) String(cleanLine ...bool) string {

	numRows := len(*cs)

	// Create new simple table
	formats := make([]string, 14)
	formats[2] = "%5d"
	formats[7] = "%5d"
	formats[9] = "%3d"
	formats[10] = "%3d"
	formats[13] = "%.3f"
	st := new(stable.Stable).Lines().
		Aligns(0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1).
		Formats(formats...)
	if numRows > 1 {
		st.Totals(&ChannelStatistic{}, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)
		numRows = 1
	}
	if len(cleanLine) > 0 && cleanLine[0] {
		st.CleanLine()
	}

	// Create and return channel statistic table in string
	return st.StructToTable(*cs)
}

// NumRows return number of rows in stringlifyed statistic
func (cs *ChannelsStatistic) NumRows() (numRows int) {
	numRows = len(*cs)
	if numRows > 1 {
		numRows++ // totals
	}
	if numRows > 0 {
		numRows += 3 // lines
		numRows += 1 // title
	}
	return
}

// StatisticPrint print tru statistics continously
func (tru *Tru) StatisticPrint() {
	tru.printStatistic(!*stathide)
}

func (tru *Tru) printStatistic(prnt bool) {

	// getLog return messages log: str - log in strung, n - number lines in log
	var getLog = func(tableLen int) (str string, n int) {
		_, h, err := tru.getTermSize()

		from := 0
		l := tru.statMsgs.len()
		if err == nil {
			from = l - (h - tableLen)
			if from < 0 {
				from = 0
			}
		}
		for i := from; i < l; i++ {
			str += term.Func.ClearLine() + tru.statMsgs.get(i) + "\n"
		}
		n = l - from
		return
	}

	// getStat return string with stat header, table and logs
	var getStat = func() (str string) {

		// Statistic to table in string
		table, numRows := tru.statToString(true)

		// Header terminal commands
		str += term.Func.SaveCursorPosition()
		str += term.Func.SetCursorPosition(1, 1)
		str += term.Func.WrapOff()

		// Table and title
		str += fmt.Sprintf(term.Func.ClearLine()+"TRU %s, RCH: %d, SCH: %d, run time: %v\n"+
			term.Func.ClearLine()+"%s\n"+
			term.Func.ClearLine(),
			tru.LocalAddr().String(),
			len(tru.readerCh),
			len(tru.senderCh),
			time.Since(tru.start),
			table,
		)

		// Log with main messages
		var n int
		if tru.statMsgs.showit() {
			var msglog string
			msglog, n = getLog(numRows + 4)
			str += term.Func.ClearLine() + "\n"
			str += msglog
			n += 1
		}

		// Footer terminal command
		str += term.Func.ClearLine() + "\n"
		str += term.Func.SetScrollRegion(numRows + 3 + n)
		str += term.Func.RestoreCursorPosition()
		str += term.Func.WrapOn()

		return
	}

	// Print statistic every 500 ms
	tru.mu.Lock()
	defer tru.mu.Unlock()
	tru.statTimer = time.AfterFunc(500*time.Millisecond, func() {
		str := getStat()
		if prnt {
			fmt.Print(str)
		}
		tru.printStatistic(prnt) // print next frame
	})
}

// StatisticPrintStop stop print statistic
func (tru *Tru) StatisticPrintStop() {
	tru.mu.Lock()
	defer tru.mu.Unlock()

	if tru.statTimer != nil {
		tru.statTimer.Stop()
		str := term.Func.ResetScrollRegion()
		str += term.Func.RestoreCursorPosition()
		fmt.Print(str)
		tru.statTimer = nil
	}
}

// StatisticMinilog start /stop show statistic minilog
func (tru *Tru) StatisticMinilog() bool {
	return tru.statMsgs.showSwap()
}

// StatisticPrintRunning return true if Statistic Print running now
func (tru *Tru) StatisticPrintRunning() bool {
	tru.mu.RLock()
	defer tru.mu.RUnlock()
	return tru.statTimer != nil
}

// statToString get and return channels statistic table in string
func (tru *Tru) statToString(cleanLine bool) (table string, numRows int) {

	// Get statistic
	stat := tru.Statistic()
	table = stat.String(true)
	numRows = stat.NumRows()

	return
}

// getTermSize get terminal size
func (tru *Tru) getTermSize() (width, height int, err error) {
	width, height, err = term.GetSize(0)
	return
}
