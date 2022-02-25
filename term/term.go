// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package term provides support functions for dealing with terminals, as
// commonly found on UNIX systems.
//
// The Getch function is the most interesting:
//
// 	fmt.Println("Press any key to continue...")
// 	term.Getch()
//
// See the getch example.
package term

import (
	"fmt"
	"os"
)

// ColorFuncs contains functions which retun escape sequences of terminal colors
// that can be written to the terminal in order to achieve different ColorFuncs
// styles of text.
type ColorFuncs struct {
}

// Color contains functions which retun escape sequences of terminal colors that
// can be written to the terminal in order to achieve different ColorFuncs
// styles of text.
var Color ColorFuncs

// None return escape sequences of Clear terminal color
func (c ColorFuncs) None() string { return "\033[0m" }

// Black return escape sequences of terminal Black color
func (c ColorFuncs) Black() string { return "\033[22;30m" }

// Red return escape sequences of terminal Red color
func (c ColorFuncs) Red() string { return "\033[22;31m" }

// Green return escape sequences of terminal Green color
func (c ColorFuncs) Green() string { return "\033[22;32m" }

// Brown return escape sequences of terminal Brown color
func (c ColorFuncs) Brown() string { return "\033[22;33m" }

// Blue return escape sequences of terminal Blue color
func (c ColorFuncs) Blue() string { return "\033[22;34m" }

// Magenta return escape sequences of terminal Magenta color
func (c ColorFuncs) Magenta() string { return "\033[22;35m" }

// Cyan return escape sequences of terminal Cyan color
func (c ColorFuncs) Cyan() string { return "\033[22;36m" }

// Grey return escape sequences of terminal Grey color
func (c ColorFuncs) Grey() string { return "\033[22;37m" }

// DarkGrey return escape sequences of terminal DarkGrey color
func (c ColorFuncs) DarkGrey() string { return "\033[01;30m" }

// LightRed return escape sequences of terminal LightRed color
func (c ColorFuncs) LightRed() string { return "\033[01;31m" }

// LightGreen return escape sequences of terminal LightGreen color
func (c ColorFuncs) LightGreen() string { return "\033[01;32m" }

// Yellow return escape sequences of terminal Yellow color
func (c ColorFuncs) Yellow() string { return "\033[01;33m" }

// LightBlue return escape sequences of terminal LightBlue color
func (c ColorFuncs) LightBlue() string { return "\033[01;34m" }

// LightMagenta return escape sequences of terminal LightMagenta color
func (c ColorFuncs) LightMagenta() string { return "\033[01;35m" }

// LightCyan return escape sequences of terminal LightCyan color
func (c ColorFuncs) LightCyan() string { return "\033[01;36m" }

// White return escape sequences of terminal White color
func (c ColorFuncs) White() string { return "\033[01;37m" }

// SetColor add selectet color escape sequences to text string and return it
func SetColor(color string, txt string) string { return color + txt + Color.None() }

// FuncFuncs contains escape sequences of terminal functions that can be written to
// the terminal in order to execute it
type FuncFuncs struct {
}

// Func contains functions which retun escape sequences of terminal functions
// that can be written to the terminal in order to execute it
var Func FuncFuncs

// Clear return 'clear terminal screen' escape sequences that can be written to
// the terminal in order to clear terminal screen
func (f FuncFuncs) Clear() string { return "\033[2J" }

// WrapOn return 'wrap lines mode' escape sequences that can be written to
// the terminal in order to set wrap terminals lines mode
func (f FuncFuncs) WrapOn() string { return "\033[?7h" }

// WrapOff return 'no wrap lines mode' escape sequences that can be written to
// the terminal in order to set no wrap terminals lines mode
func (f FuncFuncs) WrapOff() string { return "\033[?7l" }

// SaveCursorPosition return 'save cursor position' terminal escape sequences
// that can be written to the terminal in order to save cursor position
func (f FuncFuncs) SaveCursorPosition() string { return "\033[s" }

// RestoreCursorPosition return 'restore cursor position' terminal escape
// sequences that can be written to the terminal in order to restore cursor
// position
func (f FuncFuncs) RestoreCursorPosition() string { return "\033[u" }

// SetCursorPosition return 'set cursor position' terminal escape sequences
// that can be written to the terminal in order to set terminal cursor
// position
func (f FuncFuncs) SetCursorPosition(x, y int) string { return fmt.Sprintf("\033[%d;%df", y, x) }

// HideCursor return 'hide cursor' terminal escape sequences that can be
// written to the terminal in order to hide terminal cursor
func (f FuncFuncs) HideCursor() string { return "\033[?25l" }

// ShowCursor return 'show cursor' terminal escape sequences that can be
// written to the terminal in order to show terminal cursor (which was hide)
func (f FuncFuncs) ShowCursor() string { return "\033[?25h" }

// ClearLine return 'clear line' terminal escape sequences that can be
// written to the terminal in order to clear line
func (f FuncFuncs) ClearLine() string { return "\033[K" }

// SetScrollRegion return 'set scroll region' terminal escape sequences that
// can be written to the terminal in order to set scroll region
func (f FuncFuncs) SetScrollRegion(startRow int, numRows ...int) string {
	if len(numRows) == 0 || numRows[0] == 0 {
		return fmt.Sprintf("\033[%d;r", startRow)
	}
	return fmt.Sprintf("\033[%d;%dr", startRow, numRows[0])
}

// ResetScrollRegion return 'reset scroll region' terminal escape sequences that
// can be written to the terminal in order to reset scroll region
func (f FuncFuncs) ResetScrollRegion() string { return "\033[r" }

// KeysFuncs contains escape sequences of terminal functions that can be written to
// the terminal in order to execute it
type KeysFuncs struct {
}

// Keys contains functions which retun key codes returned by Getch function
var Keys KeysFuncs

// CtrlC return Ctrl+C key code
func (k KeysFuncs) CtrlC() []byte {
	return []byte{3}
}

// CtrlD return Ctrl+D key code
func (k KeysFuncs) CtrlD() []byte {
	return []byte{4}
}

// Left return arrow left key code
func (k KeysFuncs) Left() []byte {
	return []byte{27, 91, 68}
}

// State contains the state of a terminal.
type State struct {
	state
}

// IsTerminal returns whether the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	return isTerminal(fd)
}

// GetSize returns the visible dimensions of the given terminal.
//
// These dimensions don't include any scrollback buffer height.
func GetSize(fd int) (width, height int, err error) {
	return getSize(fd)
}

// Getch waits terminal key pressed and return it code
func Getch() []byte {

	oldState, err := MakeNonBlock(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer RestoreState(int(os.Stdin.Fd()), oldState)

	bytes := make([]byte, 3)
	numRead, err := os.Stdin.Read(bytes)
	if err != nil {
		return nil
	}

	return bytes[0:numRead]
}

// Anykey waits until a key is pressed
func Anykey() { Getch() }

// MakeNonBlock puts the terminal connected to the non block mode and returns
// the previous state of the terminal so that it can be restored
func MakeNonBlock(fd int) (*State, error) {
	return makeNonBlock(fd)
}

// GetState returns the current state of a terminal which may be useful to
// restore the terminal after a signal.
func GetState(fd int) (*State, error) {
	return getState(fd)
}

// RestoreState restores the terminal connected to the given file descriptor to a
// previous state.
func RestoreState(fd int, oldState *State) error {
	return restoreState(fd, oldState)
}
