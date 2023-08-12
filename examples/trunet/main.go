// Using tru with standard golang net.Listener and net.Conn interfaces
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teonet-go/tru"
)

const protocol = "tru"

var addr = flag.String("addr", ":7070", "local address")
var conn = flag.String("a", "", "remote address to connect to")
var delay = flag.Int("delay", 2000000, "send delay in Microseconds")
var nomsg = flag.Bool("nomsg", false, "dont show send receive messages")

func main() {

	// Print logo message
	fmt.Println("Trunet sample application ver. 0.0.1")

	// Parse flags
	flag.Parse()

	// Run client
	if len(*conn) > 0 {
		client(*conn)
		return
	}

	// Run server
	server(*addr)
}

// Trunet server
func server(addr string) (err error) {

	// Server mode logo
	fmt.Println("Server mode")

	// Listen announces on the local network address.
	listener, err := tru.Listen(protocol, addr)
	if err != nil {
		log.Println("can't start tru listner, error: ", err)
		return
	}
	log.Println("start listening at addr:", addr)

	// Graceful exit from application when Ctrl+C pressed
	onControlCPressed(func() {
		listener.Close()
		os.Exit(0)
	})

	for {
		// Accept an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new connection from addr:", conn.RemoteAddr())

		// Handle the connection in a separate goroutine.
		go func(conn net.Conn) {
			defer conn.Close()
			for {
				// Create a buffer for incoming data.
				buf := &bytes.Buffer{}

				// Read data from the connection.
				_, err := io.Copy(buf, conn)
				if err != nil {
					log.Println("got error:", err)
					break
				}
				logmsg("got message:", buf.Bytes())

				// Send command answer to the connection.
				res := []byte(fmt.Sprintf("done: %s", buf))
				_, err = conn.Write(res)
				if err != nil {
					log.Println(err)
				}
				logmsg("send answer:", res)

			}
			log.Println("connection closed from addr:", conn.RemoteAddr())
		}(conn)
	}
}

// Trunet client
func client(addr string) (err error) {

	// Client mode logo
	fmt.Print("Client mode")

	// Calculate send delay depent of -delay application parameter
	sendDelay := time.Microsecond * time.Duration(*delay)
	if *delay > 0 {
		fmt.Printf(", set send delay: %v\n", sendDelay)
	} else {
		fmt.Println()
	}

	// Dial to server and send messages during connection
	for {
		// Connect to server
		conn, err := tru.Dial(protocol, addr)
		if err != nil {
			log.Println("can't tru dial, error:", err)
			time.Sleep(time.Second * 5)
			continue
		}
		log.Println("connected to addr:", addr)

		// Graceful exit from application when Ctrl+C pressed
		onControlCPressed(func() {
			conn.Close()
			os.Exit(0)
		})

		// Read answers from client
		go func() {
			for {
				data, err := io.ReadAll(conn)
				if err != nil {
					log.Println("read err:", err)
					break
				}
				logmsg("got answer:", data)
			}
		}()

		// Send messages while connected
		for i := 0; ; i++ {
			data := []byte(fmt.Sprintf("Hello message # %d", i))
			_, err := conn.Write(data)
			if err != nil {
				log.Println("write err:", err)
				break
			}
			logmsg("send message:", data)

			// data, err = io.ReadAll(conn)
			// if err != nil {
			// 	log.Println("read err:", err)
			// 	break
			// }
			// logmsg("got answer:", data)

			time.Sleep(sendDelay)
		}

		log.Println("connection closed to addr:", conn.RemoteAddr())
		conn.Close()
	}
}

// Show log message depend of nomsg parameter
func logmsg(msg string, data []byte) {
	if !*nomsg {
		log.Println(msg, string(data))
	}
}

// React to Ctrl+C
func onControlCPressed(f func()) {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		for range c {
			// sig is a ^C, handle it
			f()
		}
	}()
}
