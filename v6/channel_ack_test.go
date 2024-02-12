package tru

import (
	"fmt"
	"testing"
	"time"
)

func TestAck(t *testing.T) {

	ch := Channel{}
	ack := ch.newAck(nil)

	ackPacket := &headerPacket{uint32(1), pAck}

	for i := 0; i < 256; i++ {
		pac := &headerPacket{uint32(i), pAck}
		ack.write(pac)
	}

	ack.write(ackPacket)
	ack.write(ackPacket)

	time.Sleep(ackWait)
	ack.close()
}

type aPacks []*headerPacket

func (a *aPacks) add(p *headerPacket) { *a = append(*a, p) }
func (a *aPacks) bytes() (data []byte) {
	for _, p := range *a {
		d, _ := p.MarshalBinary()
		data = append(data, d...)
	}
	return
}
func (a *aPacks) len() (l int) { return len(*a) * headerLen }

func TestAckCombine(t *testing.T) {

	ch := Channel{}
	ack := ch.newAck(nil)

	// Ack packets
	var pacs aPacks

	// Combine 10 Ack packets data
	numPacks := 100
	for id := 0; id < numPacks; id++ {
		p := &headerPacket{uint32(id), pAck}
		pacs.add(p)
		if id > 0 && (id%(numPacks/10) == 0) {
			p := &headerPacket{uint32(id + numPacks), pAck}
			pacs.add(p)
			// fmt.Println("ack packets data:", data)
		}
	}
	fmt.Println("ack packets" , len(pacs), "data len:", pacs.len())

	ack.acks = pacs
	res := ack.combine()
	for _, r := range res {
		fmt.Println("\ngot result len:", len(r), "data:", r)
	}

}
