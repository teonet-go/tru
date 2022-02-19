package tru

import (
	"testing"

	"github.com/kirill-scherba/tru/teolog"
)

func TestReceiveQueue(t *testing.T) {

	log := teolog.New()
	// log.SetLevel(teolog.Connect)
	log.Info.Println("\n\n==== TestReceiveQueue started ====")

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
	defer tru2.Close()

	// tru2 connect to tru1
	ch, err := tru2.Connect(tru1Addr)
	if err != nil {
		t.Errorf("can't connect to tru1, err: %s", err)
		return
	}

	// add packet to receive queue
	add := func(id int) {
		pac := tru2.newPacket().SetID(id)
		ch.recvQueue.add(pac)
	}

	// Add packets to receive queue
	ch.expectedID = 0
	add(1)
	add(4)
	add(3)
	add(2)
	if ch.recvQueue.len() != 4 {
		t.Errorf("wrong queue length, len = %d", ch.recvQueue.len())
		return
	}

	// Set expexted id, process queue and wait queue empty
	ch.expectedID = 1
	ch.recvQueue.process(ch, func(ch *Channel, pac *Packet) {})
	if ch.recvQueue.len() != 0 {
		t.Errorf("queue not empty, len = %d", ch.recvQueue.len())
		return
	}

	// Test recive queue with serve func
	serve := func(id int) {
		data, _ := tru2.newPacket().SetStatus(statusData).SetID(id).MarshalBinary()
		tru2.serve(0, ch.Addr(), data)
	}
	ch.expectedID = 0
	ch.stat.drop = 0
	serve(1)
	serve(4)
	serve(1)
	serve(2)
	serve(4)
	serve(2)
	if ch.recvQueue.len() != 3 {
		t.Errorf("wrong queue length(2), len = %d", ch.recvQueue.len())
		return
	}
	if ch.stat.drop != 3 {
		t.Errorf("wrong drop, drop = %d", ch.stat.drop)
		return
	}
	if ch.expectedID != 0 {
		t.Errorf("wrong expectedID, expectedID = %d", ch.expectedID)
		return
	}
	serve(3)
	serve(0)
	serve(3)
	if ch.recvQueue.len() != 0 {
		t.Errorf("wrong queue length(3), len = %d", ch.recvQueue.len())
		return
	}
	if ch.stat.drop != 4 {
		t.Errorf("wrong drop(2), drop = %d", ch.stat.drop)
		return
	}
	if ch.expectedID != 5 {
		t.Errorf("wrong expectedID(2), expectedID = %d", ch.expectedID)
		return
	}
}
