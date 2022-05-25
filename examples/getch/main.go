package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	termx "github.com/teonet-go/tru/term"
)

func getch() []byte {

	oldState, err := termx.MakeNonBlock(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer termx.RestoreState(int(os.Stdin.Fd()), oldState)

	bytes := make([]byte, 3)
	numRead, err := os.Stdin.Read(bytes)
	if err != nil {
		return nil
	}

	return bytes[0:numRead]
}

func main() {

	// Rundom output
	go func() {
		for {
			time.Sleep(2 * time.Second)
			fmt.Println("Test\nTest\nTest")
		}
	}()

L:
	for {
		c := getch()
		switch {
		case bytes.Equal(c, []byte{3}):
			fmt.Println("Ctrl+C pressed", c)
			break L
		case bytes.Equal(c, []byte{4}):
			fmt.Println("Ctrl+D pressed", c)
			break L
		case bytes.Equal(c, []byte{27, 91, 68}): // left
			fmt.Println("LEFT pressed", c)
		case bytes.Equal(c, []byte{27, 91, 67}): // right
			fmt.Println("RIGHT ! pressed", c)
		default:
			fmt.Println("Unknown pressed", c, "("+string(c)+")")
		}
	}
}
