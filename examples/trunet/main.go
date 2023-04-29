// Using tru with standard golang net.Listener and net.Conn interfaces
package main

import (
	"bytes"
	"io"
	"log"
	"net"

	"github.com/teonet-go/tru"
)

func main() {

	// Listen announces on the local network address.
	addr := "localhost:7070"
	listener, err := tru.Listen("tru", addr)
	if err != nil {
		log.Println("can't start tru listner, error: ", err)
	}
	log.Println("start listening at addr", addr)

	// select {}

	for {
		// Accept an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new connection from:", conn.RemoteAddr())

		// Handle the connection in a separate goroutine.
		go func(conn net.Conn) {
			defer conn.Close()
			for {
				// Create a buffer for incoming data.
				buf := &bytes.Buffer{}

				// Read data from the connection.
				_, err := io.Copy(buf, conn)
				// _, err := io.ReadAll(conn)
				if err != nil {
					log.Println("got error:", err)
					break
				}
				// log.Printf("got %s command: %s", conn.RemoteAddr().Network(), buf.String())

				// Execute command
				// res, err := u.command.Exec(buf.Bytes())
				// if err != nil {
				// 	res = []byte(fmt.Sprintf("error: %s\n\nUsage of commands:\n%s", err, u.command))
				// }
				res := []byte("done")

				// Send command answer to the connection.
				_, err = conn.Write(res)
				if err != nil {
					log.Println(err)
				}
				// log.Printf("send %s command answer, bytes len: %d", conn.RemoteAddr().Network(), len(res))
			}
			log.Println("connection closed from:", conn.RemoteAddr())
		}(conn)
	}
}
