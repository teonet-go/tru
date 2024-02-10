// Copyright 2023 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tru statistics module

package tru

import (
	"fmt"
	"strings"
	"time"

	"github.com/teonet-go/tru/v6/term"
)

// Reset resets statistics.
func (tru *Tru) ResetStat() {
	for chd := range tru.listChannels() {
		chd.ch.Stat.reset()
	}
	tru.started = time.Now()
}

// Printstat prints tru statistics continuously
func (tru *Tru) Printstat() {
	tru.printstat()
}

func (tru *Tru) printstat() {

	// getStat function return tru statistics string
	getStat := func() (str string) {

		// Header terminal commands
		str += term.Func.SaveCursorPosition()
		str += term.Func.SetCursorPosition(1, 1)
		str += term.Func.WrapOff()

		// Statistic to table in string
		// table, numRows := tru.statToString(true)
		//
		// table := "---" + term.Func.ClearLine()
		// numRows := 1
		//
		var table string
		var numRows = 0
		l := term.Func.ClearLine() + strings.Repeat("-", 125)

		for chd := range tru.listChannels() {
			table += "\n"
			table += term.Func.ClearLine() + fmt.Sprintf(
				"%-17.17s %9d %7d %6d %12d %6d %12d %7d %6d %5d %5d %5d %7.4f %7.3f",
				chd.addr, chd.ch.Stat.Sent(), chd.ch.Stat.SentSpeed(),
				chd.ch.Stat.Retransmit(), chd.ch.Stat.Ack(),
				chd.ch.Stat.Ackd(),
				chd.ch.Stat.Recv(), chd.ch.Stat.RecvSpeed(),
				chd.ch.Stat.Drop(), chd.ch.sq.len(),
				chd.ch.rq.len(), 0,
				float64(chd.ch.senddelay.Nanoseconds())/1000000.0,
				float64(chd.ch.Triptime().Microseconds())/1000.0,
			)
			numRows++
		}
		if numRows > 0 {
			title := "" +
				"ADDR                   SEND    SSEC   RSND          ACK   ACKD" +
				"         RECV    RSEC   DROP    SQ    RQ   RTA   DELAY      TT\n"
			table = l + "\n" + title + l + table + "\n" + l
			numRows += 4
		} else {
			table = l
			numRows = 1
		}

		// Table and title
		str += fmt.Sprintf(""+
			term.Func.ClearLine()+"TRU %s, RCH: %d, SCH: %d, run time: %v\n"+
			term.Func.ClearLine()+"%s\n"+
			term.Func.ClearLine(),

			"local", // tru.LocalAddr().String(),
			len(tru.readChannel),
			0, // len(tru.senderCh),
			time.Since(tru.started),
			table,
		)

		// Log with main messages
		var n int

		// Footer terminal command
		str += term.Func.ClearLine() + "\n"
		str += term.Func.SetScrollRegion(numRows + 3 + n)
		str += term.Func.RestoreCursorPosition()
		str += term.Func.WrapOn()

		return
	}

	// Print statistic every 500 ms
	str := getStat()
	fmt.Print(str)
	time.AfterFunc(500*time.Millisecond, tru.printstat)
}
