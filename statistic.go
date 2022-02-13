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
}

const (
	checkInactiveAfter      = 1 * time.Second
	pingInactiveAfter       = 4 * time.Second
	disconnectInactiveAfter = 5 * time.Second
)

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

// PrintStatistic print tru statistics
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

// statToString create and return channel statistic table in string
func (tru *Tru) statToString(cleanLine bool) (table string, numRows int) {

	// Simple table data structure
	type statData struct {
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

	// Create new simple table
	formats := make([]string, 12)
	formats[2] = "%5d"
	formats[5] = "%5d"
	formats[11] = "%.3f"
	st := new(stable.Stable).Lines().
		Aligns(0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1).
		Formats(formats...)
	if tru.numChannels() > 1 {
		st.Totals(&statData{}, 0, 1, 1, 1, 1, 1, 1)
		numRows = 1
	}
	if cleanLine {
		st.CleanLine()
	}

	// Add channels statistic stat slice
	tru.m.RLock()
	var stat []statData
	for _, ch := range tru.cannels {
		numRows++
		var sec = int64(time.Since(ch.stat.started).Seconds())
		var rta int
		pac := ch.sendQueue.getFirst()
		if pac != nil {
			rta = pac.retransmitAttempts
		}
		stat = append(stat, statData{
			Addr: ch.addr.String(),
			Send: ch.stat.send,
			Ssec: func() int64 {
				if sec == 0 {
					return 0
				}
				return ch.stat.send / sec
			}(),
			Rsnd: ch.stat.retransmit,
			Recv: ch.stat.recv,
			Rsec: func() int64 {
				if sec == 0 {
					return 0
				}
				return ch.stat.recv / sec
			}(),
			Drop:  ch.stat.drop,
			SQ:    uint(ch.sendQueue.len()),
			RQ:    uint(ch.recvQueue.len()),
			RTA:   rta,
			Delay: ch.delay,
			TT:    float64(ch.getTripTime().Microseconds()) / 1000.0,
		})
	}
	tru.m.RUnlock()

	// Sort slice with channels statistic by address
	sort.Slice(stat, func(i, j int) bool {
		return stat[i].Addr < stat[j].Addr
	})

	// Create and return channel statistic table in string
	table = st.StructToTable(stat)
	if numRows > 0 {
		numRows += 3 // lines
		numRows += 1 // title
	}
	return
}

func (tru *Tru) StopPrintStatistic() {
	if tru.statTimer != nil {
		tru.statTimer.Stop()
		fmt.Print("\033[?25h") // show cursor
		fmt.Print("\033[?7h")  // wrap
	}
}

func (tru *Tru) getTermSize() (width, height int, err error) {
	width, height, err = term.GetSize(0)
	return
}
