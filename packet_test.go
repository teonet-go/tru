package tru

import (
	"testing"

	"github.com/teonet-go/tru/teolog"
)

// Distance func test
func TestDistance(t *testing.T) {

	// Max uint32 = 0xFFFFFFFF

	const maxPacketNum = 0xFFFFFFFF

	log := teolog.New()
	log.SetLevel(teolog.Debug)

	var pac Packet
	var d int

	d = pac.distance(0, maxPacketNum)
	log.Debug.Println(d)

	d = pac.distance(maxPacketNum, 0)
	log.Debug.Println(d)

	d = pac.distance(0xFFFF, 0)
	log.Debug.Println(d)

	d = pac.distance(0, 0xFFFF)
	log.Debug.Println(d)

	d = pac.distance(1000, 2000)
	log.Debug.Println(d)

	d = pac.distance(10, maxPacketNum)
	log.Debug.Println(d)

}

func TestPacStatusID(t *testing.T) {

	log := teolog.New()
	log.SetFlags(0)
	log.SetLevel(teolog.Debug)

	var maxid uint32 = packetIDLimit - 1
	log.Debug.Printf("max    id: %08X, %d", maxid, maxid)
	log.Debug.Print()

	pacUnpac := func(pac *Packet) {
		// pack status and id
		statid := pac.packStatID()
		log.Debug.Printf("pack   id: %05X, stsus: %02X, statid: %08X",
			pac.id, pac.status, statid)

		// unpack status and id
		var pacout Packet
		pacout.unpackStatID(statid)
		log.Debug.Printf("unpack id: %05X, stsus: %02X, statid: %08X",
			pacout.id, pacout.status, statid)

		log.Debug.Print()

		// compare
		if pacout.id != pac.id || pacout.status != pac.status {
			t.Error("wromg pac/unpack")
		}
	}

	pacUnpac(&Packet{id: 48, status: 11})
	pacUnpac(&Packet{id: maxid, status: 255})
}
