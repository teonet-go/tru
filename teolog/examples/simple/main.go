package main

import (
	"fmt"
	"log"

	"github.com/teonet-go/tru/teolog"
)

func main() {

	stdFlags := log.LstdFlags | log.Lmicroseconds

	// Init log
	log := teolog.New()

	// After create all log message available
	fmt.Println("1) show info, debug and debug-v messages:")
	log.Info.Println("info message print")
	log.Debug.Println("debug message print")
	log.Debugv.Println("debug-v message print")

	// Change default log flags (remove file name)
	fmt.Println("2) set flags:")
	log.SetFlags(stdFlags)
	log.Info.Println("info message print")
	log.Debug.Println("debug message print")
	log.Debugv.Println("debug-v message print")

	// Set log level to Debug: Debug messages and low (Error User Info Connect)
	// will be printed
	fmt.Println("3) set level:")
	log.SetLevel(teolog.Debug)
	log.Info.Println("info message print")
	log.Debug.Println("debug message print")
	log.Debugv.Println("debug-v message print")
}
