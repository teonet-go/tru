// Print in the same line sample application
package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"
)

func main() {

	// Signal processing
	var stop bool
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, handle it
			fmt.Println("sig:", sig)
			stop = true
		}
	}()

	// Random source
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	// before entering the loop
	fmt.Print("\033[s") // save the cursor position

	var err error = nil
	for i := 0; ; i++ {
		// fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
		fmt.Print("\033[u") // restore the cursor position

		if err != nil {
			fmt.Printf("Could not retrieve file info for %d\n", i)
		} else {
			fmt.Printf("Retrieved-1: %d %2d\n", i, r1.Intn(100))
			fmt.Printf("Retrieved-2: %d %2d\n", i, r1.Intn(100))
			fmt.Printf("Retrieved-3: %d %v\n", i, r1.Perm(10))
			r1.Float64()
		}
		time.Sleep(250 * time.Millisecond)
		if stop {
			break
		}
	}
}
