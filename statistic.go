// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channel statistic module

package tru

import (
	"fmt"
	"sort"
	"time"

	"github.com/kirill-scherba/stable"
	"golang.org/x/term"
)

type statistic struct {
	destroyed          bool
	started            time.Time
	lastActivity       time.Time
	lastSend           time.Time
	lastDelayCheck     time.Time
	checkActivityTimer *time.Timer

	tripTime      time.Duration
	tripTimeMidle time.Duration
	send          int64
	retransmit    int64
	recv          int64
	drop          int64
	sendSpeed     speed
	recvSpeed     speed
}

const (
	checkInactiveAfter      = 1 * time.Second
	pingInactiveAfter       = 4 * time.Second
	disconnectInactiveAfter = 5 * time.Second
)

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
	s.checkActivityTimer.Stop()
	s.sendSpeed.destroy()
	s.recvSpeed.destroy()
	s.destroyed = true
}

// setSend set one packet send
func (s *statistic) setSend() {
	s.send++
	s.sendSpeed.add()
}

// setRecv set one packet received
func (s *statistic) setRecv() {
	s.recv++
	s.recvSpeed.add()
}

// setLastActivity set channels last activity time
func (s *statistic) setLastActivity() {
	s.lastActivity = time.Now()
}

// checkActivity check channel activity every second and call inactive func
// if channel inactive time grate than disconnectInactiveAfter time constant,
// and call keepalive func if channel inactive time grate than pingInactiveAfter
func (s *statistic) checkActivity(inactive, keepalive func()) {
	s.checkActivityTimer = time.AfterFunc(checkInactiveAfter, func() {
		switch {

		case time.Since(s.lastActivity) > disconnectInactiveAfter:
			inactive()
			return

		case time.Since(s.lastActivity) > pingInactiveAfter:
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
	tru.m.RLock()
	defer tru.m.RUnlock()

	// Get rta function
	rta := func(ch *Channel) (rta int) {
		if pac := ch.sendQueue.getFirst(); pac != nil {
			rta = pac.retransmitAttempts
		}
		return
	}

	// Append channels statistic to slice
	for _, ch := range tru.cannels {
		stat = append(stat, ChannelStatistic{
			Addr:  ch.addr.String(),
			Send:  ch.stat.send,
			Ssec:  int64(ch.stat.sendSpeed.get()),
			Rsnd:  ch.stat.retransmit,
			Recv:  ch.stat.recv,
			Rsec:  int64(ch.stat.recvSpeed.get()),
			Drop:  ch.stat.drop,
			SQ:    uint(ch.sendQueue.len()),
			RQ:    uint(ch.recvQueue.len()),
			RTA:   rta(ch),
			Delay: ch.delay,
			TT:    float64(ch.getTripTime().Microseconds()) / 1000.0,
		})
	}

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
	formats := make([]string, 12)
	formats[2] = "%5d"
	formats[5] = "%5d"
	formats[7] = "%3d"
	formats[8] = "%3d"
	formats[11] = "%.3f"
	st := new(stable.Stable).Lines().
		Aligns(0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1).
		Formats(formats...)
	if numRows > 1 {
		st.Totals(&ChannelStatistic{}, 0, 1, 1, 1, 1, 1, 1)
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

// PrintStatistic print tru statistics continously
func (tru *Tru) PrintStatistic() {

	var printStat func()
	var start = time.Now()
	fmt.Print("\033c")    // clear screen
	fmt.Print("\033[s")   // save the cursor position
	fmt.Print("\033[?7l") // no wrap

	// Print stat log list
	var printLog = func(tableLen int) {
		_, h, err := tru.getTermSize()

		from := 0
		l := len(tru.statLogMsgs)
		if err == nil {
			from = l - (h - tableLen)
			if from < 0 {
				from = 0
			}
		}
		for i := from; i < l; i++ {
			msg := tru.statLogMsgs[i]
			fmt.Println("\033[K" + msg)
		}
	}

	// Print stat header, table and logs
	var print = func() {
		table, numRows := tru.statToString(true)
		fmt.Print("\033[?25l") // hide cursor
		fmt.Print("\033[u")    // restore the cursor position
		// "\033[K" - clear line
		fmt.Printf("\033[KTRU %s, RCH: %d, SCH: %d, run time: %v\n\033[K%s\n\033[K",
			tru.LocalAddr().String(),
			len(tru.readerCh),
			len(tru.senderCh),
			time.Since(start),
			table,
		)
		fmt.Println("\033[K")
		printLog(numRows + 3)
	}

	// Print statistic every 250 ms
	printStat = func() {
		tru.statTimer = time.AfterFunc(500*time.Millisecond, func() {
			print()
			printStat()
		})
	}

	printStat()
}

// StopPrintStatistic stop print statistic
func (tru *Tru) StopPrintStatistic() {
	if tru.statTimer != nil {
		tru.statTimer.Stop()
		fmt.Print("\033[?25h") // show cursor
		fmt.Print("\033[?7h")  // wrap
	}
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
