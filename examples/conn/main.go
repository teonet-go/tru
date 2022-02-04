// Create UDP connection and send packets client/server sample application
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")

func main() {

	// Parse flags
	flag.Parse()

	// Create server connection
	pc, err := Create(*port)
	if err != nil {
		log.Fatal(err)
	}

	// Listen server data
	go Listen(pc)

	// Connect to remote server and send data packets
	go Sender(pc, *addr)

	select {}
}

func Create(ports ...int) (pc net.PacketConn, err error) {
	var port int
	if len(ports) > 0 {
		port = ports[0]
	}
	pc, err = net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	return
}

// Listen to incoming udp packets
func Listen(pc net.PacketConn) {
	defer func() {
		log.Printf("stop listen\n")
		pc.Close()
	}()
	log.Printf("start listen at %s\n", pc.LocalAddr().String())

	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		// go serve(pc, addr, buf[:n])
		log.Printf("got %d from %s, data: %s\n", n, addr.String(), buf[:n])
	}
}

// Sender data to remote server
func Sender(pc net.PacketConn, addr string) {
	if addr == "" {
		return
	}
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; ; i++ {
		data := []byte(fmt.Sprintf("data %d", i))
		n, err := pc.WriteTo(data, a)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("send %d to %s, data: %s\n", n, a.String(), data[:n])

		time.Sleep(3 * time.Second)
	}
}
