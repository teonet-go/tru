package tru

import (
	"bufio"
	"fmt"
	"os"

	"github.com/kirill-scherba/tru/hotkey"
	"github.com/kirill-scherba/tru/teolog"
	"github.com/kirill-scherba/tru/term"
)

// hotkeyMenuState is hotkey menu state
type hotkeyMenuState struct {
	logLevel teolog.LogLevel
}

// stopLog stop logger while next key pressed
func (s *hotkeyMenuState) stopLog(h *hotkey.Hotkey) teolog.LogLevel {
	state := h.State().(*hotkeyMenuState)
	state.logLevel = log.Level() // get current log level
	log.SetLevel(0)              // stop log
	h.NextKeyAction(func(ch []byte) {
		log.SetLevel(state.logLevel)
	})
	return state.logLevel
}

// restoreLog continue stopped by stopLog() logger with saved log level
func (s *hotkeyMenuState) restoreLog(h *hotkey.Hotkey) teolog.LogLevel {
	state := h.State().(*hotkeyMenuState)
	log.SetLevel(state.logLevel)
	return state.logLevel
}

// conitnueLog continue stopped by stopLog() logger with current log level
func (s *hotkeyMenuState) continueLog(h *hotkey.Hotkey) teolog.LogLevel {
	h.NextKeyAction(nil)
	return log.Level()
}

// newHotkey create and start run tru hokey menu
func (tru *Tru) newHotkey() *hotkey.Hotkey {
	quit := func(h *hotkey.Hotkey) {
		h.Stop()
		fmt.Println("Quit from application...")
		tru.Close()
		os.Exit(0)
	}
	state := new(hotkeyMenuState)
	h := hotkey.New().SetState(state).Run().

		// Add action function which executes when unknown key pressed
		AddUnknown(func(h *hotkey.Hotkey, ch []byte) {
			fmt.Printf(" unknown key %v pressed, use 'h' key to get help\n", ch)
		}).

		// When Enter key pressed than print empty line
		Add([]byte{13}, "", func(h *hotkey.Hotkey) {
			fmt.Println()
		}).

		// Help print usage test
		Add([]string{"h", "?"}, "show this help screen", func(h *hotkey.Hotkey) {
			h.State().(*hotkeyMenuState).stopLog(h)
			fmt.Print(tru.Logo("", ""))                                               // Logo
			fmt.Print(h)                                                              // Menu
			fmt.Println("\n select menu item hotkey or press any key to continue...") // Footer

		}).

		// Show or Hide tru statistic
		Add("u", "show/hide tru statistic", func(h *hotkey.Hotkey) {
			if !tru.StatisticPrintRunning() {
				tru.StatisticPrint()
				fmt.Println(" print statistic started...")
			} else {
				tru.StatisticPrintStop()
				fmt.Println(" print statistic stopped...")
			}
		}).

		// Show or Hide tru statistic minilog
		Add("U", "show/hide tru statistic minilog", func(h *hotkey.Hotkey) {
			var s string
			if tru.StatisticMinilog() {
				s = "show"
			} else {
				s = "hide"
			}

			fmt.Printf(" tru statistic minilog %s\n", s)
		}).

		// Switch log level
		Add("l", "switch logger level", func(h *hotkey.Hotkey) {
			log.SwitchLevel()
			fmt.Println(" logger set to level:", log)
		}).

		// Print current logger level
		Add("L", "print current logger level", func(h *hotkey.Hotkey) {
			level := h.State().(*hotkeyMenuState).stopLog(h)
			fmt.Println(" current logger level:", level.String())
		}).

		// Set log level
		Add(hotkey.KeyCode{Code: []byte{12}, Name: "Ctrl+L"},
			"set new logger level", func(h *hotkey.Hotkey) {

				level := h.State().(*hotkeyMenuState).stopLog(h)

				fmt.Println(log.Levels())

				fmt.Println(" current logger level:", level.String())
				reader := bufio.NewReader(os.Stdin)
				fmt.Print(" enter new level ~> ")
				text, _ := reader.ReadString('\n')
				log.SetLevel(text)
				h.State().(*hotkeyMenuState).continueLog(h)
			}).

		// Set log filter
		Add("f", "set logger filter", func(h *hotkey.Hotkey) {
			level := h.State().(*hotkeyMenuState).stopLog(h)
			fmt.Println(" current log level: ", level.String())
			fmt.Println(" current log filter:", log.Filter())
			reader := bufio.NewReader(os.Stdin)
			fmt.Print(" enter new filter ~> ")
			text, _ := reader.ReadString('\n')
			log.SetFilter(text)
			h.State().(*hotkeyMenuState).restoreLog(h)
		}).

		// Quit form hotkey menu
		Add(hotkey.KeyCode{Code: term.Keys.CtrlC(), Name: "Ctrl+C"},
			"stop this hotkey menu", func(h *hotkey.Hotkey) {
				h.Stop()
				fmt.Println("Hotkey menu stopped...")
			},
		).

		// Quit from application
		Add("q", "quit from application", quit)

	fmt.Println("Hotkey menu startted, press 'h' to help...")
	return h
}
