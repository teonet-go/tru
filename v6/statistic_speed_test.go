package tru

import (
	"fmt"
	"testing"
	"time"
)

func TestSpeedPeriod(t *testing.T) {

	s := NewSpeed()

	// Spleet second for 10 periods
	for i := 0; i < 15; i++ {
		part := time.Now().Nanosecond() / 100000000
		s.Add()
		fmt.Println(part, s.Speed())
		time.Sleep(time.Millisecond * 100)
	}

}
