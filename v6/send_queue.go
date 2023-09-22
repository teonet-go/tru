package tru

import (
	"container/list"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// sendQueue data structure and methods receiver.
//
// Send queue store sent packets data and sent time untile the answer packet
// received.
type sendQueue struct {
	m             sendQueueMap // Send queue map by packet id
	l             list.List    // List of elements in order of arrival
	wait                       // Waits while front element has retransmits
	*sync.RWMutex              // Mutex to protect parameters
}
type sendQueueMap map[uint32]*list.Element

// sendQueueData stores send packet data, sent time and number of retransmits.
type sendQueueData struct {
	data []byte                    // Sent packet
	at   atomic.Pointer[time.Time] // Sent time
	r    uint64                    // Number of retransmits
}

// incRetransmit increments the number of packet retransmit
func (sqd *sendQueueData) incRetransmit() { atomic.AddUint64(&sqd.r, 1) }

// retransmit gets the number of packet retransmit
func (sqd *sendQueueData) retransmit() uint64 { return atomic.LoadUint64(&sqd.r) }

// setTime sets the sent packet time
func (sqd *sendQueueData) setTime(t time.Time) { sqd.at.Store(&t) }

// time gets the sent packet time
func (sqd *sendQueueData) time() time.Time { return *sqd.at.Load() }

// wait uses to wait during send packet while front element of send queue has
// retransmits
type wait struct {
	*sync.Mutex
	*sync.Cond
}

// init initialize wait structure
func (w *wait) init() {
	w.Mutex = new(sync.Mutex)
	w.Cond = sync.NewCond(w.Mutex)
}

// newSendQueue creates a new send queue object
func newSendQueue() *sendQueue {
	sq := new(sendQueue)
	sq.wait.init()
	sq.m = make(sendQueueMap)
	sq.RWMutex = new(sync.RWMutex)
	return sq
}

// add adds packet to send queue
func (sq *sendQueue) add(id uint32, data []byte) {
	sq.Lock()
	defer sq.Unlock()

	packet := &sendQueueData{data: data}
	packet.setTime(time.Now())
	sq.m[id] = sq.l.PushBack(packet)
}

// del removes packet from send queue by id
func (sq *sendQueue) del(id uint32) (data *sendQueueData, ok bool) {
	sq.Lock()
	defer sq.Unlock()

	el, ok := sq.m[id]
	if !ok {
		return
	}
	data, ok = sq.l.Remove(el).(*sendQueueData)
	delete(sq.m, id)

	// sq.Signal()

	// data, ok = el.Value.(*sendQueueData)
	return
}

// get gets packet from send queue by id
func (sq *sendQueue) get(id uint32, lock ...bool) (data *sendQueueData, ok bool) {
	if len(lock) == 0 || lock[0] {
		sq.RLock()
		defer sq.RUnlock()
	}

	el, ok := sq.m[id]
	if !ok {
		return
	}
	data, ok = el.Value.(*sendQueueData)
	return
}

// front gets front element from send queue list
func (sq *sendQueue) front() *list.Element {
	sq.RLock()
	defer sq.RUnlock()
	return sq.l.Front()
}

// next gets next element from send queue list
func (sq *sendQueue) next(el *list.Element) *list.Element {
	sq.RLock()
	defer sq.RUnlock()
	return el.Next()
}

// len returns send queue length
func (sq *sendQueue) len() int {
	sq.RLock()
	defer sq.RUnlock()
	return len(sq.m)
}

// writeDelay uses in send packets and wait whail send will be avalable
func (sq *sendQueue) writeDelay(id uint32) {
	// sq.L.Lock()
	// defer sq.L.Unlock()

	const sleepTime = 8 * time.Microsecond
	for {

		// Wait if id already in send queue
		if _, ok := sq.get(id); ok {
			time.Sleep(sleepTime)
			continue
		}

		// Get front element
		el := sq.front()
		if el == nil {
			return
		}

		// Get sendQueueData
		sqd, ok := el.Value.(*sendQueueData)
		if !ok {
			// TODO: wrong element can't be in send queue
			return
		}

		// Check packet retransmit counter
		if sqd.retransmit() < 1 {
			return
		}

		// Wait until can write
		// sq.Wait()
		// continue

		time.Sleep(sleepTime)
		break
	}
}

// process send queue
func (sq *sendQueue) process(conn net.PacketConn, ch *Channel) {
	if ch.closed() {
		// log.Printf("send queue process of channel %s stopped", ch.addr)
		return
	}

	var i int
	var tt = ch.Triptime()
	const extraTime = 10 * time.Millisecond

	// sq.RLock()
	for el := sq.front(); el != nil; el = sq.next(el) {
	// for el := sq.l.Front(); el != nil; el = el.Next() {
		// Get send queue data
		sqd, ok := el.Value.(*sendQueueData)
		if !ok {
			// TODO: This wrong element shoud be deleted
			log.Printf("bad packet in send queue\n")
			continue
		}

		// Stop if time since sent packet time less than triptime + extraTime
		if time.Since(sqd.time()) <= tt+extraTime {
			break
		}

		// Stop if second element has retransmits
		if i > 0 && sqd.retransmit() > 0 {
			break
		}

		// Retransmit package
		conn.WriteTo(sqd.data, ch.addr)
		ch.Stat.incRetransmit()
		sqd.setTime(time.Now())
		sqd.incRetransmit()

		i++
	}
	// sq.RUnlock()

	time.AfterFunc(tt*1+0*time.Millisecond, func() { sq.process(conn, ch) })
}
