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
		Addr string  // peer address
		Send int64   // send packets
		Ssec int64   // send per second
		Rsnd int64   // resend packets
		Recv int64   // receive packets
		Rsec int64   // receive per second
		Drop int64   // drop received packets
		SQ   uint    // send queue length
		RQ   uint    // receive queue length
		TT   float64 // trip time
	}

	go func() {
		start := time.Now()
		fmt.Print("\033[s") // save the cursor position
		for {
			time.Sleep(250 * time.Millisecond)

			var stat []statData
			// var aligns = []int{0, 0, 0, 1}
			var st = new(stable.Stable)

			tru.m.RLock()
			for _, ch := range tru.cannels {
				sec := int64(time.Since(ch.stat.started).Seconds())
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
					Drop: ch.stat.drop,
					SQ:   uint(ch.sendQueue.len()),
					RQ:   uint(ch.recvQueue.len()),
					TT:   float64(ch.stat.tripTime.Microseconds()) / 1000.0,
				})
			}
			tru.m.RUnlock()

			sort.Slice(stat, func(i, j int) bool {
				return stat[i].Addr < stat[j].Addr
			})

			fmt.Print("\033[?25l") // hide cursor
			fmt.Print("\033[u")    // restore the cursor position
			fmt.Printf("\033[KTRU %s, RCh: %d, SCh: %d, time: %v:\n%s\n\033[K",
				tru.LocalAddr().String(),
				len(tru.readerCh),
				len(tru.senderCh),
				time.Since(start),

				st.StructToTable(stat), // aligns...),
			)
			fmt.Print("\033[?25h") // show cursor
		}
	}()
}
