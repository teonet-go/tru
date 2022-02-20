// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Teolog message logger extends standart go log with levels. There is there
// next levels from hi to low:
// NONE, ERROR, USER, INFO, CONNECT, DEBUG, DEBUGV, DEBUGVV, DEBUGVVV
package teolog

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type Teolog struct {
	Error    *log.Logger
	User     *log.Logger
	Info     *log.Logger
	Connect  *log.Logger
	Debug    *log.Logger
	Debugv   *log.Logger
	Debugvv  *log.Logger
	Debugvvv *log.Logger
	levels   []*log.Logger
	level    int
}

const (
	None = iota
	Error
	User
	Info
	Connect
	Debug
	Debugv
	Debugvv
	Debugvvv
)

// New create new teolog
func New() (teolog *Teolog) {
	flag := log.LstdFlags | log.Lmsgprefix | log.Lshortfile | log.Lmicroseconds
	teolog = &Teolog{
		Error:    log.New(os.Stdout, "[erro] ", flag),
		User:     log.New(os.Stdout, "[user] ", flag),
		Info:     log.New(os.Stdout, "[info] ", flag),
		Connect:  log.New(os.Stdout, "[conn] ", flag),
		Debug:    log.New(os.Stdout, "[debu] ", flag),
		Debugv:   log.New(os.Stdout, "[debv] ", flag),
		Debugvv:  log.New(os.Stdout, "[devv] ", flag),
		Debugvvv: log.New(os.Stdout, "[dvvv] ", flag),
	}
	teolog.levels = append(teolog.levels,
		teolog.Error, teolog.User, teolog.Info, teolog.Connect,
		teolog.Debug, teolog.Debugv, teolog.Debugvv, teolog.Debugvvv,
	)

	return
}

func (l *Teolog) SetFlags(flag int) {
	for _, tl := range l.levels {
		tl.SetFlags(flag | log.Lmsgprefix)
	}
}

// SetLevel set log level. There is there next levels from hi to low:
// NONE, ERROR, USER, INFO, CONNECT, DEBUG, DEBUGV, DEBUGVV, DEBUGVVV
func (l *Teolog) SetLevel(leveli interface{}) {
	var level int
	switch v := leveli.(type) {
	case int:
		level = v
	case string:
		level = l.levelFromStr(v)
	}
	l.level = level
	for i, tl := range l.levels {
		if i < level {
			tl.SetOutput(os.Stdout)
		} else {
			tl.SetOutput(ioutil.Discard)
		}
	}
}

func (l *Teolog) levelFromStr(level string) int {
	switch strings.ToLower(level) {
	case "none":
		return None
	case "error":
		return Error
	case "user":
		return User
	case "info":
		return Info
	case "connect":
		return Connect
	case "debug":
		return Debug
	case "debugv":
		return Debugv
	case "debugvv":
		return Debugvv
	case "debugvvv":
		return Debugvvv
	default:
		return None
	}
}
