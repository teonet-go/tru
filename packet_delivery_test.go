package tru

import (
	"testing"
	"time"

	"github.com/teonet-go/tru/teolog"
)

func TestPacketDelivery(t *testing.T) {

	log := teolog.New()
	// log.SetLevel(teolog.Connect)
	log.Info.Println("\n\n==== TestPacketDelivery started ====")

	// create tru1
	tru1, err := New(0, log)
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

	// Create wait channel
	wait := make(chan interface{})
	defer close(wait)

	// Send packet fron tru2 to tru1 and get delivery callback
	ch.WriteTo([]byte("some test data"), func(pac *Packet, err error) {
		if err != nil {
			t.Errorf("can't receive delivery callback from tru1, err: %s", err)
			return
		}
		log.Debugvvv.Println("got delivery answer to packet id", pac.ID())
		wait <- nil
	})
	<-wait

	// Send packet fron tru2 to tru1, close tru1 connection and get delivery
	// callback timeout
	ch.WriteTo([]byte("some test data"), func(pac *Packet, err error) {
		if err == nil {
			t.Errorf("can't receive delivery callback timeout from tru1, err: %s", err)
			return
		}
		log.Debugvvv.Println("got delivery timeout to packet id", pac.ID())
		wait <- nil
	}, 50*time.Millisecond)
	ch.Close()
	<-wait
}
