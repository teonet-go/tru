package main

import (
	"fmt"
	"os"

	"github.com/kirill-scherba/tru/hotkey"
	"github.com/kirill-scherba/tru/term"
)

func main() {
	fmt.Println("Teonet terminal hotkey sample application ver. 0.0.1")

	fmt.Println("Press hotkey, use h to get help")

	hotkey.New().Run().

		// Help menu
		Add([]string{"h", "?"}, "help", func(h *hotkey.Hotkey) {
			fmt.Print(h)
		}).

		// Test action
		Add("t", "test action", func(h *hotkey.Hotkey) {
			fmt.Println("This is test action!")
		}).

		// Statistic action
		Add("u", "show statistic", func(h *hotkey.Hotkey) {
			fmt.Println("This is statistic action!")
		}).

		// Quit menu
		Add([]hotkey.KeyCode{
			{Code: []byte("q")},
			{Code: term.Keys.CtrlC(), Name: "^C"},
		}, "quit from hotkey menu", func(h *hotkey.Hotkey) {
			fmt.Println("Quit...")
			h.Stop()
			os.Exit(0)
		}).

		// Left key pressed
		Add(hotkey.KeyCode{Code: term.Keys.Left()},
			"left key action menu", func(h *hotkey.Hotkey) {
				fmt.Println("Left key pressed...")
			},
		)

	select {}
}
