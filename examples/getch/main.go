package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	termx "github.com/kirill-scherba/tru/term"
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

// Returns either an ascii code, or (if input is an arrow) a Javascript key code.
// func getChar() []byte {
// 	t, _ := term.Open("/dev/tty")
// 	term.RawMode(t)
// 	bytes := make([]byte, 3)

// 	numRead, err := t.Read(bytes)
// 	if err != nil {
// 		return nil
// 	}

// 	t.Restore()
// 	t.Close()
// 	return bytes[0:numRead]
// }

// func getch2() []byte {

// 	// Disable input buffering
// 	exec.Command("stty", "-F", "/dev/tty", "-icanon", "min", "1").Run()
// 	// Disable echo, and interrupt
// 	exec.Command("stty", "-F", "/dev/tty", "-echo", "-isig").Run() // "-echoctl",
// 	// Enable input buffering, echo and interrupt
// 	defer exec.Command("stty", "-F", "/dev/tty", "icanon", "echo", "isig").Run()

// 	bytes := make([]byte, 3)

// 	numRead, err := os.Stdin.Read(bytes)
// 	if err != nil {
// 		return nil
// 	}

// 	return bytes[0:numRead]
// }

func main() {

	// Rundom output
	go func() {
		for {
			time.Sleep(2 * time.Second)
			fmt.Println("Test\nTest\nTest")
		}
	}()

	// Method 1
	// if 1 == 0 {
	// 	// disable input buffering
	// 	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	// 	// do not display entered characters on the screen
	// 	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

	// 	var b []byte = make([]byte, 3)
	// 	for {
	// 		n, _ := os.Stdin.Read(b)
	// 		fmt.Println("I got the byte", b[:n], "("+string(b[:n])+")")
	// 		if string(b[:n]) == "q" {
	// 			break
	// 		}
	// 	}
	// 	exec.Command("stty", "-F", "/dev/tty", "echo").Run()
	// }

	// Method 2
L:
	for {
		// c := getch()
		c := getch()
		switch {
		case bytes.Equal(c, []byte{3}):
			fmt.Println("Ctrl+C pressed", c)
			// return
			break L
		case bytes.Equal(c, []byte{4}):
			fmt.Println("Ctrl+D pressed", c)
			//return
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
