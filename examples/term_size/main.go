package main

import "golang.org/x/term"

func main() {
	if term.IsTerminal(0) {
		println("in a term")
	} else {
		println("not in a term")
	}
	width, height, err := term.GetSize(0)
	if err != nil {
		return
	}
	println("width:", width, "height:", height)
}
