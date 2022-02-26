// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Teolog message logger extends standart go log with levels. There is there
// next levels from hi to low:
// NONE, ERROR, USER, INFO, CONNECT, DEBUG, DEBUGV, DEBUGVV, DEBUGVVV
package teolog

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// Log levels constant
const (
	None     LogLevel = iota // No logs
	Error                    // Errors log (show only errors)
	User                     // User level log (show User logs and Error logs)
	Info                     // Info level (show Info, User and Error logs)
	Connect                  // Connect level (show connect, Info, User and Error logs)
	Debug                    // Debug level (show debug, connect, Info, User and Error logs)
	Debugv                   // Debugv level (show debugv, debug, connect, Info, User and Error logs)
	Debugvv                  // Debugvv level (show debugvv, debugv, debug, connect, Info, User and Error logs)
	Debugvvv                 // Debugvvv level (show debugvvv, debugvv, debugv, debug, connect, Info, User and Error logs)
	numLevels
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
	level    LogLevel
	filter   Filter
	writer   writer
}

type LogLevel int

func (l LogLevel) String() string {
	return new(Teolog).levelToStr(l)
}

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

// Levels return levels description
func (l *Teolog) Levels() string {
	return l.levelsToStr()
}

// SetLevel set log level. There is there next levels from hi to low:
// NONE, ERROR, USER, INFO, CONNECT, DEBUG, DEBUGV, DEBUGVV, DEBUGVVV
func (l *Teolog) SetLevel(leveli interface{}) {
	var level LogLevel
	switch v := leveli.(type) {
	case int:
		level = LogLevel(v)
	case LogLevel:
		level = v
	case string:
		level = l.levelFromStr(v)
	}
	l.level = level
	for i, tl := range l.levels {
		if LogLevel(i) < level {
			tl.SetOutput(l.writer) // os.Stdout)
		} else {
			tl.SetOutput(ioutil.Discard)
		}
	}
}

// Level return current log level
func (l *Teolog) Level() LogLevel {
	return l.level
}

// String return current log level in string
func (l *Teolog) String() string {
	return l.levelToStr(l.level)
}

// SwitchLevel switch level to nexet and return level number
func (l *Teolog) SwitchLevel() LogLevel {
	l.level++
	if l.level >= numLevels {
		l.level = 0
	}
	l.SetLevel(l.level)
	return l.level
}

// SetFilter set log filter
func (l *Teolog) SetFilter(f interface{}) {

	switch v := f.(type) {
	case Filter:
		l.filter = v
	case string:
		l.filter = Logfilter(v)
	default:
		panic("wrong filter tipe")
	}

}

// ClearFilter clear log filter
func (l *Teolog) ClearFilter() {
	l.filter = []string{}
}

// Filter return log filter
func (l *Teolog) Filter() (str string) {
	for i := range l.filter {
		if i > 0 {
			str += " || "
		}
		str += l.filter[i]
	}
	return
}

// levelFromStr get level from string
func (l *Teolog) levelFromStr(level string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
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

// levelToStr return current log level in string
func (l *Teolog) levelToStr(level LogLevel) string {
	switch level {
	case None:
		return "None"
	case Error:
		return "Error"
	case User:
		return "User"
	case Info:
		return "Info"
	case Connect:
		return "Connect"
	case Debug:
		return "Debug"
	case Debugv:
		return "DebugV"
	case Debugvv:
		return "DebugVV"
	case Debugvvv:
		return "DebugVVV"
	default:
		return "Wrong level"
	}
}

// levelsToStr return levels description
func (l *Teolog) levelsToStr() (str string) {
	var f = " %-10s %s\n"
	str += "\n List of available logger levels:\n\n"
	str += fmt.Sprintf(f, "None", "No logs")
	str += fmt.Sprintf(f, "Error", "Errors log")
	str += fmt.Sprintf(f, "User", "User level log")
	str += fmt.Sprintf(f, "Info", "Info level")
	str += fmt.Sprintf(f, "Connect", "Connect")
	str += fmt.Sprintf(f, "Debug", "Debug level")
	str += fmt.Sprintf(f, "Debugv", "Debugv level")
	str += fmt.Sprintf(f, "Debugvv", "Debugvv level")
	str += fmt.Sprintf(f, "Debugvvv", "Debugvvv level")
	return
}

// Filter teonet log filter type
type Filter []string

func (lf Filter) trim() {
	for i := range lf {
		lf[i] = strings.TrimSpace(lf[i])
	}
}

// Logfilter make log filter from string
func Logfilter(str string) (f Filter) {
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
