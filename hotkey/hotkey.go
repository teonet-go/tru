// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hotkey provides support functions for managing hotkeys in
// terminal/console application.
//
// Function Add is the most common requirement:
//
// 	h := hotkey.New()
// 	h.Add("H", func() { fmt.Println("This is Help") })
//
// Note that on non-Unix systems os.Stdin.Fd() may not be 0 and Getch() may run
// incorrectly
package hotkey

import (
	"errors"
	"fmt"
	"sort"

	"github.com/teonet-go/tru/term"
)

type Hotkey struct {
	hotkeys       map[string]*hotkeyData
	unknownKey    func(h *Hotkey, ch []byte)
	nextKeyAction func(ch []byte)
	state         interface{}
	stop          bool
	stopactions   []func()
}

type hotkeyData struct {
	keys        []KeyCode
	description string
	action      func(h *Hotkey)
}

// KeyCode define key code and key name which hows in hokey meny
type KeyCode struct {
	Code []byte
	Name string
}

// name return KeyCode name
func (hk KeyCode) name() string {
	if len(hk.Name) > 0 {
		return hk.Name
	}
	return string(hk.Code)
}

// New Create new hotkey menu
func New() *Hotkey {
	h := &Hotkey{}
	h.hotkeys = make(map[string]*hotkeyData)
	h.unknownKey = defUnknownKey
	return h
}

// SetState set users menu state. State is some interface which user can
// use in its hotkey menu. State is place where user can save its own menu
// parameters
func (h *Hotkey) SetState(state interface{}) *Hotkey {
	h.state = state
	return h
}

// State return saved user menu state
func (h *Hotkey) State() interface{} {
	return h.state
}

// NextKeyAction set action which execute when key pressed nex time
func (h *Hotkey) NextKeyAction(f func(ch []byte)) {
	h.nextKeyAction = f
}

// Add hotkey menu
func (h *Hotkey) Add(keys interface{}, description string, action func(h *Hotkey)) *Hotkey {

	var keysAr []KeyCode
	switch v := keys.(type) {
	case []byte:
		keysAr = append(keysAr, KeyCode{Code: v})
	case string:
		keysAr = append(keysAr, KeyCode{Code: []byte(v)})
	case []string:
		for i := range v {
			keysAr = append(keysAr, KeyCode{Code: []byte(v[i])})
		}
	case KeyCode:
		keysAr = append(keysAr, v)
	case []KeyCode:
		for i := range v {
			keysAr = append(keysAr, v[i])
		}
	default:
		err := errors.New("wrong type of letters parameter")
		panic(err)
	}

	hd := &hotkeyData{keysAr, description, action}
	for _, l := range keysAr {
		h.hotkeys[string(l.Code)] = hd
	}
	return h
}

// AddUnknown add action function which executes when unknown key pressed
func (h *Hotkey) AddUnknown(action func(h *Hotkey, ch []byte)) *Hotkey {
	h.unknownKey = action
	return h
}

// defUnknownKey default action when unknown key pressed
func defUnknownKey(h *Hotkey, ch []byte) {
	fmt.Println("unknown key pressed", ch)
}

// String return string contains hotkey menu help
func (h *Hotkey) String() (str string) {
	var ar []*hotkeyData
	// find in hotkeyData slice
	find := func(hd *hotkeyData) bool {
		for _, v := range ar {
			if v == hd {
				return true
			}
		}
		return false
	}
	// add to hotkeyData slice if does not exists
	add := func(hd *hotkeyData) {
		if find(hd) {
			return
		}
		ar = append(ar, hd)
	}
	// sort hotkeyData slice by first hotkey
	sort := func() {
		sort.Slice(ar, func(i, j int) bool {
			return ar[i].keys[0].name() < ar[j].keys[0].name()
		})
	}

	// Add hotkeyData to slice and sort it
	for i := range h.hotkeys {
		add(h.hotkeys[i])
	}
	sort()

	// Add hotkeys and description to returns string
	for i := range ar {
		var keys interface{}
		if len(ar[i].keys) > 1 {
			k := []string{}
			for _, key := range ar[i].keys {
				k = append(k, key.name())
			}
			keys = fmt.Sprintf("%v", k)
		} else {
			keys = ar[i].keys[0].name()
		}
		str += fmt.Sprintf("   %-10s %s\n", keys, ar[i].description)
	}

	return
}

// Run execute hotkey menu, wait key pressed and execute it action
func (h *Hotkey) Run() *Hotkey {
	go h.run()
	return h
}

// Stop execute hotkey menu
func (h *Hotkey) Stop() {
	for _, f := range h.stopactions {
		f()
	}
	h.stop = true
}

// SetStopAction set callback function which will run when hotkey menu Stop
func (h *Hotkey) SetStopAction(f func()) {
	h.stopactions = append(h.stopactions, f)
}

// run execute hotkey menu, wait key pressed and execute it action
func (h *Hotkey) run() {
	for !h.stop {
		ch := term.Getch()
		if h.nextKeyAction != nil {
			h.nextKeyAction(ch)
			h.nextKeyAction = nil
		}
		hd, ok := h.hotkeys[string(ch)]
		if !ok {
			if h.unknownKey != nil {
				h.unknownKey(h, ch)
			}
			continue
		}
		if hd.action == nil {
			continue
		}
		hd.action(h)
	}
	h.stop = false
}
