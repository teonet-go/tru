// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU Channel statistic module

package tru

import (
	"time"
)

type statistic struct {
	destroyed          bool
	lastActivity       time.Time
	checkActivityTimer *time.Timer
}

const (
	checkInactiveAfter      = 1 * time.Second
	disconnectInactiveAfter = 5 * time.Second
)

// setLastActivity set channels last activity time
func (s *statistic) setLastActivity() {
	s.lastActivity = time.Now()
}

// checkActivity check channel activity every second and call inactive func
// if channel inactive more than disconnectInactiveAfter time constant
func (s *statistic) checkActivity(inactive func()) {
	s.checkActivityTimer = time.AfterFunc(checkInactiveAfter, func() {
		if time.Since(s.lastActivity) > disconnectInactiveAfter {
			inactive()
			return
		}
		s.checkActivity(inactive)
	})
}
