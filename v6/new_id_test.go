package tru

import (
	"fmt"
	"net"
	"sync"
	"testing"
)

// Test new newExpectedId function when id > packetIDLimit
func TestNewExpectedId(t *testing.T) {

	const runIt = false
	if !runIt {
		return
	}

	var conn net.PacketConn
	var addr string
	ch, _ := newChannel(conn, addr)

	ch.expId = packetIDLimit - 10
	var wg sync.WaitGroup
	for i := 0; i < 20000000; i++ {
		wg.Add(1)
		go func(i int) {
			oldId := ch.expId // ch.expectedId()
			newId := ch.newExpectedId()
			if oldId >= packetIDLimit || oldId == packetIDLimit-1 && newId > 0 {
				fmt.Printf("oldId(i=%d) = %d\n", i, oldId)
				if oldId >= packetIDLimit {
					fmt.Printf("oldId(i=%d) by expectedId() = %d\n", i, oldId%packetIDLimit)
				}
				fmt.Printf("newId(i=%d) = %d\n", i, newId)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}
