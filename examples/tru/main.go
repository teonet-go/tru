// Create UDP connection and send packets with tru package client/server sample
// application
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/kirill-scherba/tru"
	"github.com/kirill-scherba/tru/teolog"
)

var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")
var loglevel = flag.String("loglevel", "", "set log level")
var logfilter = flag.String("logfilter", "", "set log filter")
var stat = flag.Bool("stat", false, "print statistic")
var hotkey = flag.Bool("hotkey", false, "start hotkey menu")
var delay = flag.Int("delay", 0, "send delay in Microseconds")
var sendlen = flag.Int("sendlen", 0, "send packet data length")
var datalen = flag.Int("datalen", 0, "set max data len in created packets, 0 - maximum UDP len")

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

var log = teolog.New()

func main() {

	// Print logo message
	fmt.Println("TRU sample application ver. 0.0.1")

	// Parse flags
	flag.Parse()

	// CPU and memory profiles
	cpuMemoryProfiles()

	// Create server connection and start listen incominng packets
	tru, err := tru.New(*port, Reader, tru.Stat(*stat),
		tru.Hotkey(*hotkey), log, *loglevel, teolog.Logfilter(*logfilter))
	if err != nil {
		log.Error.Fatal("can't create tru, err: ", err)
	}
	defer tru.Close()
	tru.SetMaxDataLen(*datalen)

	// Set default send delay
	// tru.SetSendDelay(*delay)

	// Send packets if addr flag set
	go Sender(tru, *addr)

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
	if err != nil {
		log.Debug.Println("got error in main reader:", err)
		return
	}
	log.Debugv.Printf("got %d byte from %s, id %d: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
	ch.WriteTo(append([]byte("answer to "), pac.Data()...))
	return
}

// Sender send data to remote server
func Sender(t *tru.Tru, addr string) {
	if addr == "" {
		return
	}

connect:
	log.Debug.Println("connect to peer", addr)
	ch, err := t.Connect(addr, func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
		if err != nil {
			log.Debug.Println("got error in channel reader, err:", err)
			return
		}
		log.Debugv.Printf("got %d byte from %s, id %d: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
		return true
	})
	if err != nil {
		log.Connect.Println(err)
		time.Sleep(5 * time.Second)
		goto connect
	}

	for i := 0; ; i++ {

		var data []byte
		if *sendlen == 0 {
			data = []byte(fmt.Sprintf("data %d", i))
		} else {
			data = make([]byte, *sendlen)
			rand.Read(data)
		}

		_, err := ch.WriteTo(data)
		if err != nil {
			log.Connect.Println(err)
			goto connect
		}
		log.Debugv.Printf("send %d bytes data to %s, data: %s\n", len(data), ch.Addr().String(), data)

		time.Sleep(time.Duration(*delay) * time.Microsecond)
	}
}

func cpuMemoryProfiles() {
	// CPU profiller
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Debugvvv.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Debugvvv.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Memory profiler
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Debugvvv.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Debugvvv.Fatal("could not write memory profile: ", err)
		}
	}
}
