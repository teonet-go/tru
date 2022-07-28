package tru

import (
	"fmt"
	"testing"

	"github.com/teonet-go/tru/teolog"
)

func TestPacketSend(t *testing.T) {

	log := teolog.New()
	log.SetLevel(teolog.Debug)
	log.Info.Println("\n\n==== TestPacketSend started ====")

	// Create wait channel
	wait := make(chan interface{})
	numPackets := 10000
	recvPackets := 0

	// Reader read packets from connected peers
	reader := func(ch *Channel, pac *Packet, err error) (processed bool) {
		if err != nil {
			log.Debug.Println("got error in main reader:", err)
			return
		}
		// fmt.Printf(".")
		// fmt.Printf("got %d byte from %s, id %d: %s\n", pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
		ch.WriteTo(append([]byte("answer to "), pac.Data()...))
		recvPackets++
		if recvPackets == numPackets {
			wait <- nil
		}
		return
	}

	// create tru1
	tru1, err := New(0, reader, log)
	if err != nil {
		t.Errorf("can't start tru1, err: %s", err)
		return
	}
	defer tru1.Close()
	tru1Addr := tru1.LocalAddr().String()

	// create tru2
	tru2, err := New(0, log)
	if err != nil {
		t.Errorf("can't start tru2, err: %s", err)
		return
	}

	// Connect tru2 to tru1
	ch, err := tru2.Connect(tru1Addr)
	if err != nil {
		t.Errorf("can't connect to tru1, err: %s", err)
		return
	}
	defer tru2.Close()
	tru2.StatisticPrint()

	// Send multy packets
	data := make([]byte, 1500)
	for i := 0; i < numPackets; i++ {

		// Send packet fron tru2 to tru1 and get delivery callback
		// []byte(fmt.Sprintf("some test data id %d", i)
		_, err = ch.WriteTo(data)
		if err != nil {
			t.Errorf("WriteTo err: %s", err)
			return
		}

	}

	fmt.Println("All send, wait receiving...")
	<-wait
	fmt.Println("All received")
}
