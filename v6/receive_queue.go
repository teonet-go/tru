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
	rq := new(receiveQueue)
	rq.m = make(map[uint32]*receiveQueueData)
	return rq
}

// add packet to receive queue
func (rq *receiveQueue) add(id uint32, data []byte) (err error) {
	rq.Lock()
	defer rq.Unlock()

	if _, ok := rq.get(id, false); ok {
		err = errPackedIdAlreadyExists
		return
	}

	rq.m[id] = &receiveQueueData{data: data}
	return
}

// del packet from receive queue
func (rq *receiveQueue) del(id uint32) (data []byte, ok bool) {
	rq.Lock()
	defer rq.Unlock()

	data, ok = rq.get(id, false)
	if ok {
		delete(rq.m, uint32(id))
	}
	return
}

// get packet from receive queue
func (rq *receiveQueue) get(id uint32, lock ...bool) (data []byte, ok bool) {
	if len(lock) == 0 || lock[0] {
		rq.RLock()
		defer rq.RUnlock()
	}

	rqd, ok := rq.m[id]
	if ok {
		data = rqd.data
	}
	return
}

// len return receive queue len
func (rq *receiveQueue) len() int {
	rq.RLock()
	defer rq.RUnlock()

	return len(rq.m)
}

// process finds packets in receive queue, returts paket data and ok if found,
// and remove packet from receive queue
// func (rq *receiveQueue) process(ch *Channel) (data []byte, ok bool) {
// 	id := ch.expectedId()
// 	data, ok = rq.del(id)
// 	return
// }
