// Main Tru V6 sample application
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/teonet-go/tru/v6"
)

const (
	appName    = "Tru V6 main sample application"
	appShort   = "tru-v6"
	appVersion = "v6.0.0"
)

var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")
var stat = flag.Bool("stat", false, "print statistic")
var nomsg = flag.Bool("nomsg", false, "don't show send receive messages")

// var ch *tru.Channel

func main() {

	// Print logo message
	fmt.Printf("%s ver. %s\n", appName, appVersion)

	// Parse flags
	flag.Parse()

	// Set log flags
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	// if *nomsg {
	// 	log.SetOutput(io.Discard)
	// }

	// Create tru object
	tru := tru.New(*stat)

	// Create connection
	conn, err := Create(tru, *port)
	if err != nil {
		fmt.Printf("can't start listener: %s\n", err)
		return
	}

	// Listen server data
	go Listen(conn)

	// Connect to remote server and send data packets
	go Sender(tru, conn, *addr)

	select {}
}

// Create server connection and start listening
func Create(tru *tru.Tru, ports ...int) (conn net.PacketConn, err error) {
	var port int
	if len(ports) > 0 {
		port = ports[0]
	}
	conn, err = tru.ListenPacket("udp", fmt.Sprintf(":%d", port))
	return
}

// Listen to incoming udp packets
func Listen(conn net.PacketConn) {
	defer func() {
		log.Printf("stop listen\n")
		conn.Close()
	}()
	log.Printf("start listen at %s\n", conn.LocalAddr().String())

	for {
		data := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(data)
		if err != nil {
			log.Printf("error reading from: %s\n", err)
			continue
		}

		if !*nomsg {
			log.Printf("got %d from %s, data: %s\n", n, addr.String(), data[:n])
		}
	}
}

// Sender sends data to remote server
func Sender(tru *tru.Tru, conn net.PacketConn, addr string) {
	if addr == "" {
		return
	}
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	for i := 0; ; i++ {

		data := []byte(fmt.Sprintf("data %d", i))
		n, err := conn.WriteTo(data, a)
		if err != nil {
			log.Fatal(err)
		}

		if !*nomsg {
			log.Printf("send %d to %s, data: %s\n", n, a.String(), data[:n])
		}

		// time.Sleep(1 * time.Microsecond)
		if time.Since(start) > 1000*time.Millisecond {

			ch := tru.GetChannel(a)

			fmt.Println("was send", i+1, "per second")
			fmt.Println("send queue size", ch.SendQueueLen())
			fmt.Println("---")

			for ch.SendQueueLen() > 0 {
				time.Sleep(1 * time.Millisecond)
			}
			fmt.Println("was send", i+1, "per second")
			fmt.Printf("retransmit %d, %.2f%%\n",
				ch.Retransmit(), 100.00*float64(ch.Retransmit())/float64(i+1))
			fmt.Println("got answers", ch.Answer())
			fmt.Println("send queue size", ch.SendQueueLen())
			fmt.Println("trip time ", ch.Triptime())
			fmt.Println("---")
			fmt.Println("real time", time.Since(start))

			// time.Sleep(5 * time.Second)
			// os.Exit(0)
			select {}
		}
	}
}
