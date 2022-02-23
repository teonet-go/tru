package tru

import (
	"fmt"
	"os"

	"github.com/kirill-scherba/tru/hotkey"
	"github.com/kirill-scherba/tru/term"
)

func NewHotkey(tru *Tru) *hotkey.Hotkey {
	quit := func(h *hotkey.Hotkey) {
		h.Stop()
		fmt.Println("Quit...")
		tru.Close()
		os.Exit(0)
	}
	h := hotkey.New().Run().
		Add([]byte{13}, "", func(h *hotkey.Hotkey) {
			fmt.Println()
		}).
		Add([]string{"h", "?"}, "help", func(h *hotkey.Hotkey) {
			fmt.Print(h)
		}).
		Add("q", "quit application", quit).
		Add(hotkey.KeyCode{Code: term.Keys.CtrlC(), Name: "Ctrl+C"},
			"stop this hotkey menu", func(h *hotkey.Hotkey) {
				h.Stop()
				fmt.Println("Stop hotkey menu...")
			}).
		Add("u", "show/hide tru statistic", func(h *hotkey.Hotkey) {
			if !tru.StatisticPrintRunning() {
				tru.StatisticPrint()
				fmt.Println("print statistic started...")
			} else {
				tru.StatisticPrintStop()
				fmt.Println("print statistic stopped...")
			}
		})
	fmt.Println("Hotkey menu startted, press h to help...")
	return h
}
