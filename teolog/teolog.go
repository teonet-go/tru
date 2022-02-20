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
	filter   TeologFilter
	writer   writer
}

// Log levels constant
const (
	None     = iota // No logs
	Error           // Errors log (show only errors)
	User            // User level log (show User logs and Error logs)
	Info            // Info level (show Info, User and Error logs)
	Connect         // Connect level (show connect, Info, User and Error logs)
	Debug           // Debug level (show debug, connect, Info, User and Error logs)
	Debugv          // Debugv level (show debugv, debug, connect, Info, User and Error logs)
	Debugvv         // Debugvv level (show debugvv, debugv, debug, connect, Info, User and Error logs)
	Debugvvv        // Debugvvv level (show debugvvv, debugvv, debugv, debug, connect, Info, User and Error logs)
)

// New create new teolog
func New() (teolog *Teolog) {
	flag := log.LstdFlags | log.Lmsgprefix | log.Lshortfile | log.Lmicroseconds
	teolog = &Teolog{
		Error:    log.New(ioutil.Discard, "[erro] ", flag),
		User:     log.New(ioutil.Discard, "[user] ", flag),
		Info:     log.New(ioutil.Discard, "[info] ", flag),
		Connect:  log.New(ioutil.Discard, "[conn] ", flag),
		Debug:    log.New(ioutil.Discard, "[debu] ", flag),
		Debugv:   log.New(ioutil.Discard, "[debv] ", flag),
		Debugvv:  log.New(ioutil.Discard, "[devv] ", flag),
		Debugvvv: log.New(ioutil.Discard, "[dvvv] ", flag),
	}
	teolog.levels = append(teolog.levels,
		teolog.Error, teolog.User, teolog.Info, teolog.Connect,
		teolog.Debug, teolog.Debugv, teolog.Debugvv, teolog.Debugvvv,
	)
	teolog.writer.l = teolog
	return
}

// SetFlags sets the output flags for the logger.
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
			tl.SetOutput(l.writer) // os.Stdout)
		} else {
			tl.SetOutput(ioutil.Discard)
		}
	}
}

// SetFilter set log filter
func (l *Teolog) SetFilter(f TeologFilter) {
	l.filter = f
}

// ClearFilter clear log filter
func (l *Teolog) ClearFilter() {
	l.filter = []string{}
}

// levelFromStr get level from string
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
	case "debugvvv", "all":
		return Debugvvv
	default:
		return None
	}
}

// TeologFilter teonet log filter type
type TeologFilter []string

func (lf TeologFilter) trim() {
	for i := range lf {
		lf[i] = strings.TrimSpace(lf[i])
	}
}

// Logfilter make log filter from string
func Logfilter(str string) (f TeologFilter) {
	f = strings.Split(str, "||")
	f.trim()
	return
}

// writer teonet log writer
type writer struct {
	l *Teolog
}

// Write writes len(b) bytes to the File.
// It returns the number of bytes written and an error, if any.
// Write returns a non-nil error when n != len(b).
func (w writer) Write(p []byte) (n int, err error) {
	var valid bool
	for _, f := range w.l.filter {
		if strings.Contains(string(p), f) {
			valid = true
			break
		}
	}
	if len(w.l.filter) > 0 && !valid {
		return
	}
	return os.Stdout.Write(p)
}
