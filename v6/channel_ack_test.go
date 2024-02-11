package tru

import (
	"testing"
	"time"
)

func TestAck(t *testing.T) {

	ch := Channel{}
	ack := ch.newAck(nil)

	ackPacket := []byte{1, 2, 3, 4}

	for i := 0; i < 256; i++ {
		ack.write(ackPacket)
	}

	ack.write(ackPacket)
	ack.write(ackPacket)

	time.Sleep(ackWait)
	ack.close()
}

func TestAckCombine(t *testing.T) {
}