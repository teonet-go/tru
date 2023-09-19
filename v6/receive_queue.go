package tru

import (
	"sync"
)

type receiveQueue struct {
	m            map[uint32]*receiveQueueData // Receive queue map
	sync.RWMutex                              // Receive queue mutex
}

// receiveQueueData stores received packet data, sent time and number of retransmits.
type receiveQueueData struct {
	data []byte // Received packet
}

// newReceiveQueue creates a new receive queue object
func newReceiveQueue() *receiveQueue {
	r := new(receiveQueue)
	r.m = make(map[uint32]*receiveQueueData)
	return r
}

// add packet to receive queue
func (r *receiveQueue) add(id uint32, data []byte) {
	r.Lock()
	defer r.Unlock()

	r.m[id] = &receiveQueueData{data: data}
}

// delete packet from receive queue
func (r *receiveQueue) delete(id uint32) (data []byte, ok bool) {
	r.Lock()
	defer r.Unlock()

	data, ok = r.get(id, false)
	if ok {
		delete(r.m, uint32(id))
	}
	return
}

// get packet from receive queue
func (r *receiveQueue) get(id uint32, lock ...bool) (data []byte, ok bool) {
	if len(lock) == 0 || lock[0] {
		r.RLock()
		defer r.RUnlock()
	}

	rqd, ok := r.m[id]
	if ok {
		data = rqd.data
	}
	return
}

// len return receive queue len
func (r *receiveQueue) len() int {
	r.RLock()
	defer r.RUnlock()

	return len(r.m)
}

// process finds packets in receive queue, returts paket data and ok if found, 
// and remove packet from receive queue
func (r *receiveQueue) process(ch *Channel) (data []byte, ok bool) {
	id := ch.expectedId()
	data, ok = r.delete(id)
	return
}
