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

		for addr, ch := range tru.channels {
			table += "\n"
			table += term.Func.ClearLine() + fmt.Sprintf(
				"%-17.17s %9d %7d %6d %12d %6d %12d %7d %6d %5d %5d %5d %7d %7.3f",
				addr, ch.Stat.Sent(), 0, ch.Stat.Retransmit(), ch.Stat.Ack(), 0,
				ch.Stat.Recv(), 0, ch.Stat.Drop(), ch.sq.len(), ch.rq.len(), 0,
				0, float64(ch.Triptime().Microseconds())/1000.0,
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
			0,       // len(tru.senderCh),
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
