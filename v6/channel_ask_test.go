package tru

import (
	"testing"
	"time"
)

func TestAsk(t *testing.T) {

	ch := Channel{}
	ask := ch.newAsk(nil)

	askPacket := []byte{1, 2, 3, 4}

	for i := 0; i < 256; i++ {
		ask.write(askPacket)
	}

	ask.write(askPacket)
	ask.write(askPacket)

	time.Sleep(askWait)
	ask.close()
}
