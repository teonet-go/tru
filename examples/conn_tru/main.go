// Create UDP connection and send packets with tru package client/server sample
// application
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kirill-scherba/tru"
)

var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")
var nolog = flag.Bool("nolog", false, "disable log messages")
var stat = flag.Bool("stat", false, "print statistic")
var delay = flag.Int("delay", 0, "send delay in Microseconds")

func main() {

	// Print logo message
	fmt.Println("TRU sample application ver. 0.0.1")

	// Parse flags
	flag.Parse()

	// Set log options
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	if *nolog {
		log.SetOutput(ioutil.Discard)
	}

	// Create server connection and start listen incominng packets
	tru, err := tru.New(*port, Reader)
	if err != nil {
		log.Fatal(err)
	}
	defer tru.Close()

	// Set default send delay
	tru.SetSendDelay(*delay)

	// Send packets if addr flag set
	go Sender(tru, *addr)

	// Print statistic if -stat flag is on
	if *stat {
		tru.PrintStatistic()
	}

	// React to Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	for range c {
		// sig is a ^C, handle it
		return
	}

	// select {}
}

// Reader read packets from connected peers
func Reader(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
	log.Printf("got %d byte from %s, id %d: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
	ch.WriteTo(append([]byte("answer to "), pac.Data()...))
	return
}

// Sender send data to remote server
func Sender(t *tru.Tru, addr string) {
	if addr == "" {
		return
	}

connect:
	log.Println("connect to peer", addr)
	ch, err := t.Connect(addr, func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
		log.Printf("got %d byte from %s, id %d: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
		return true
	})
	if err != nil {
		log.Println(err)
		time.Sleep(5 * time.Second)
		goto connect
	}

	for i := 0; ; i++ {
		data := []byte(fmt.Sprintf("data %d", i))
		err := ch.WriteTo(data)
		if err != nil {
			log.Println(err)
			goto connect
		}
		log.Printf("send %d bytes data to %s, data: %s\n", len(data), ch.Addr().String(), data)

		// time.Sleep(time.Duration(*delay) * time.Microsecond)
	}
}
