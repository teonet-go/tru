// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !zos && !windows && !solaris && !plan9
// +build !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!zos,!windows,!solaris,!plan9

package term

import (
	"fmt"
	"runtime"
)

type state struct{}

func isTerminal(fd int) bool {
	return false
}

func getSize(fd int) (width, height int, err error) {
	return 0, 0, fmt.Errorf("terminal: GetSize not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func makeNonBlock(fd int) (*State, error) {
	return nil, fmt.Errorf("terminal: MakeRaw not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func getState(fd int) (*State, error) {
	return nil, fmt.Errorf("terminal: GetState not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func restoreState(fd int, state *State) error {
	return fmt.Errorf("terminal: Restore not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}
