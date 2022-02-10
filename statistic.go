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
	checkActivityTimer *time.Timer

	tripTime   time.Duration
	send       int64
	retransmit int64
	recv       int64
	drop       int64
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

	start := time.Now()
	fmt.Print("\033c")    // clear screen
	fmt.Print("\033[s")   // save the cursor position
	fmt.Print("\033[?7l") // no wrap

	var printStat func()
	var printLog = func(listLen int) {
		_, h, err := tru.getTermSize()

		from := 0
		l := len(tru.statLogMsgs)
		if err == nil {
			from = l - (h - listLen)
			if from < 0 {
				from = 0
			}
		}
		for i := from; i < l; i++ {
			msg := tru.statLogMsgs[i]
			fmt.Println("\033[K" + msg)
		}
	}

	printStat = func() {
		tru.statTimer = time.AfterFunc(250*time.Millisecond, func() {

			var stat []statData
			aligns := []int{0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
			var formats = make([]string, 12)
			formats[2] = "%5d"
			formats[5] = "%5d"
			formats[11] = "%.3f"

			var st = new(stable.Stable).Lines().CleanLine(true).Aligns(aligns...).
				Formats(formats...).Totals(&statData{}, 0, 1, 1, 1, 1, 1, 1)

			tru.m.RLock()
			listLen := 0
			for _, ch := range tru.cannels {
				listLen++
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
					TT:    float64(ch.stat.tripTime.Microseconds()) / 1000.0,
				})
			}
			tru.m.RUnlock()

			sort.Slice(stat, func(i, j int) bool {
				return stat[i].Addr < stat[j].Addr
			})

			fmt.Print("\033[?25l") // hide cursor
			fmt.Print("\033[u")    // restore the cursor position
			// fmt.Print("\033[K")    // clear line
			fmt.Printf("TRU %s, RCH: %d, SCH: %d, time: %v\n\n%s\n\033[K",
				tru.LocalAddr().String(),
				len(tru.readerCh),
				len(tru.senderCh),
				time.Since(start),

				st.StructToTable(stat), // aligns...),
			)

			fmt.Println("\033[K")
			printLog(listLen + 6)

			// fmt.Print("\033[?25h") // show cursor
			printStat()
		})
	}

	printStat()
}

func (tru *Tru) StopPrintStatistic() {
	if tru.statTimer != nil {
		tru.statTimer.Stop()
		fmt.Print("\033[?25h") // show cursor
		fmt.Print("\033[?7h")  // wrap
	}
}

func (tru *Tru) getTermSize() (width, height int, err error) {

	// if term.IsTerminal(0) {
	// 	println("in a term")
	// } else {
	// 	println("not in a term")
	// }
	width, height, err = term.GetSize(0)
	return
}
