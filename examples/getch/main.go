package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"golang.org/x/term"
)

func getch() []byte {

	fmt.Println("\033[?25l")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	t := os.NewFile(os.Stdin.Fd(), "tru terminal")
	bytes := make([]byte, 3)
	numRead, err := t.Read(bytes)
	if err != nil {
		return nil
	}

	return bytes[0:numRead]
}

func main() {

L:
	for {
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
			fmt.Println("RIGHT pressed", c)
		default:
			fmt.Println("Unknown pressed", c, "("+string(c)+")")
		}
	}

	// oldState, _ := term.MakeRaw(int(os.Stdin.Fd()))
	// defer term.Restore(int(os.Stdin.Fd()), oldState)

	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	c := bufio.NewReadWriter(r, w)
	t := term.NewTerminal(c, "teo > ")
	t.AutoCompleteCallback = func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		// newLine = line
		// ok = true
		return
	}

	str, _ := t.ReadLine()

	fmt.Println("str:", str)
}
