// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU statistic log module

package tru

import (
	"fmt"
	"sync"
	"time"
)

const totalMessages = 256 // Max number of messages in statistic log slice

// statisticLog TRU statistic log module type and receiver
type statisticLog struct {
	msgs    []string
	showit_ bool
	sync.RWMutex
}

// add message to the statistic log slice
func (s *statisticLog) add(msg string) {
	s.Lock()
	defer s.Unlock()

	const layout = "2006-01-02 15:04:05.000000"
	msg = fmt.Sprintf("%v %s", time.Now().Format(layout), msg)
	s.msgs = append(s.msgs, msg)
	if len(s.msgs) > totalMessages {
		s.msgs = s.msgs[1:]
	}
}

// len returt statistic log slice len
func (s *statisticLog) len() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.msgs)
}

// get and returt statistic log slice message by index
func (s *statisticLog) get(i int) (msg string) {
	if i < 0 || i >= s.len() {
		return
	}
	return s.msgs[i]
}

// showit return flag of show minilog in statistic
func (s *statisticLog) showit() bool {
	s.RLock()
	defer s.RUnlock()

	return s.showit_
}

// showSwap swap flag of show minilog in statistic
func (s *statisticLog) showSwap() bool {
	s.Lock()
	defer s.Unlock()

	s.showit_ = !s.showit_
	return s.showit_
}
