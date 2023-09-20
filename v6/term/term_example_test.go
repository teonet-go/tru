package term_test

import (
	"fmt"

	"github.com/teonet-go/tru/v6/term"
)

func ExampleColor_Red() {
	fmt.Println(term.Color.Red() + "hello")
}

func ExampleColor_Green() {
	fmt.Println(term.Color.Green() + "hello")
}

func ExampleSetColor() {
	fmt.Println(term.SetColor(term.Color.Green(), "hello"))
}

func ExampleFunc_HideCursor() {
	fmt.Print(term.Func.HideCursor())
}

func ExampleFunc_ShowCursor() {
	fmt.Print(term.Func.ShowCursor())
}
