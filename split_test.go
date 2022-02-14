package tru

import (
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"testing"
)

func TestSplitPacket(t *testing.T) {

	// tru1 reader
	var wait = make(chan interface{})
	var recvData []byte
	reader1 := func(ch *Channel, pac *Packet, err error) (processed bool) {
		if err != nil {
			return
		}
		log.Printf("got packet id %d, data len %d", pac.ID(), len(pac.Data()))
		processed = true
		if pac.ID() == 0 {
			recvData = pac.Data()
			// wait <- nil
		}
		if pac.ID() >= 1 {
			wait <- nil
		}
		return
	}

	// create tru1
	tru1, err := New(0, reader1)
	if err != nil {
		t.Errorf("can't start tru1, err: %s", err)
		return
	}
	tru1Addr := tru1.LocalAddr().String()
	// log.SetOutput(ioutil.Discard)

	// create tru2
	tru2, err := New(0)
	if err != nil {
		t.Errorf("can't start tru2, err: %s", err)
		return
	}
	tru2.SetMaxDataLen(512)

	// tru2 connect to tru1
	ch, err := tru2.Connect(tru1Addr)
	if err != nil {
		t.Errorf("can't connect to tru1, err: %s", err)
		return
	}

	// pac := tru2.newPacket()
	var data = make([]byte, 0.5*1024*1024 /* pac.MaxDataLen()*33 */)
	rand.Read(data)
	ch.WriteTo(data)
	log.Println("write data, len", len(data))

	var data2 = make([]byte, 1024)
	rand.Read(data2)
	ch.WriteTo(data2)
	log.Println("write data, len", len(data2))

	<-wait

	// Compare send and receive buffers of long packet
	equal := reflect.DeepEqual(data, recvData)
	fmt.Println("compare buffers", equal)
	if !equal {
		t.Errorf("send and receive data buffers not equal")
	}

	// Show statistic
	stat1, _ := tru1.statToString(false)
	fmt.Println(stat1)
	fmt.Println()
	stat2, _ := tru2.statToString(false)
	fmt.Println(stat2)
}
