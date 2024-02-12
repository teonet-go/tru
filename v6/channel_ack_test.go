package tru

import (
	"fmt"
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

type aPacks [][]byte

func (a *aPacks) add(p []byte) { *a = append(*a, p) }
func (a *aPacks) bytes() (data []byte) {
	for _, p := range *a {
		data = append(data, p...)
	}
	return
}
func (a *aPacks) len() (l int) {
	for _, p := range *a {
		l += len(p)
	}
	return
}

func TestAckCombine(t *testing.T) {

	ch := Channel{}
	ack := ch.newAck(nil)

	// Ack packets
	var pacs aPacks

	// Combine 10 Ack packets data
	numPacks := 100
	for id := 0; id < numPacks; id++ {
		data, _ := headerPacket{uint32(id), pAck}.MarshalBinary()
		pacs.add(data)
		if id > 0 && (id%(numPacks/10) == 0) {
			data, _ := headerPacket{uint32(id + numPacks), pAck}.MarshalBinary()
			pacs.add(data)
			// fmt.Println("ack packets data:", data)
		}
	}
	fmt.Println("ack packets data len:", pacs.len(), pacs)

	res := ack.combine()
	fmt.Println("\ngot result, len:", len(res), "data:", res)

	out := ack.acksToBytes(res)
	fmt.Println("\ngot output, len:", ack.acksToBytesLen(out), "data:", out)
}
