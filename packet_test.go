package tru

import (
	"testing"

	"github.com/kirill-scherba/tru/teolog"
)

// Distance func test
func TestDictance(t *testing.T) {

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
