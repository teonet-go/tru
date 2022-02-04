// Create UDP connection and send packets with tru package client/server sample
// application
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kirill-scherba/tru"
)

var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")

func main() {

	// Print logo message
	log.Println("TRU sample application ver. 0.0.1")

	// Parse flags
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Create server connection and start listen incominng packets
	t, err := tru.New(*port, Reader)
	if err != nil {
		log.Fatal(err)
	}

	// Send packets if addr flag set
	Sender(t, *addr)

	select {}
}

// Reader read packets from connected peers
func Reader(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
	log.Printf("got %d byte from %s, id: %d, data: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
	ch.WriteTo(append([]byte("answer to "), pac.Data()...))
	return
}

// Sender send data to remote server
func Sender(t *tru.Tru, addr string) {
	if addr == "" {
		return
	}

	ch, err := t.Connect(addr, func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
		log.Printf("ch: got %d byte from %s, id: %d, data: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
		return true
	})
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; ; i++ {
		data := []byte(fmt.Sprintf("data %d", i))
		err := ch.WriteTo(data)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("send %d bytes data to %s, data: %s\n", len(data), ch.Addr().String(), data)

		time.Sleep(2500 * time.Millisecond)
	}
}
