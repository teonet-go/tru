package tru

import "sync/atomic"

// ChannelStat contains statistics data (atomic vars) and methods to work with it
type ChannelStat struct {
	sent       uint64 // Number of data packets sent
	sentSpeed  *Speed // Number of data packets sent per second
	ack        uint64 // Number of packets answer (acknowledgement) received
	ackd       uint64 // Number of dropped packets answer (acknowledgement)
	retransmit uint64 // Number of packets data retransmited
	retrprev   uint64 // Previouse retransmits value
	recv       uint64 // Number of data packets received
	recvSpeed  *Speed // Number of data packets received per second
	drop       uint64 // Number of dropped received data packets
}

// init initializes channel statistics object. It starts speed calculation
// processes. The ChannelStat object should be closed with close() method after
// usage.
func (ch *ChannelStat) init() {
	ch.sentSpeed = NewSpeed()
	ch.recvSpeed = NewSpeed()
}

// close closes channel statistics object.
func (ch *ChannelStat) close() {
	ch.sentSpeed.Close()
	ch.recvSpeed.Close()
}

// reset resets channel statistics.
func (ch *ChannelStat) reset() { 
	ch.close()
	*ch = ChannelStat{}
	ch.init()
 }

// Ack returns answer (acknowledgement) counter value
func (ch *ChannelStat) Ack() uint64 { return atomic.LoadUint64(&ch.ack) }

// ackd returns dropped answer (acknowledgement) counter value
func (ch *ChannelStat) Ackd() uint64 { return atomic.LoadUint64(&ch.ackd) }

// Sent returns data packet sent counter value.
func (ch *ChannelStat) Sent() uint64   { return atomic.LoadUint64(&ch.sent) }
func (ch *ChannelStat) SentSpeed() int { return ch.sentSpeed.Speed() }

// Recv returns data packet received counter value
func (ch *ChannelStat) Recv() uint64   { return atomic.LoadUint64(&ch.recv) }
func (ch *ChannelStat) RecvSpeed() int { return ch.recvSpeed.Speed() }

// Drop returns number of droppet received data packets (duplicate data packets)
func (ch *ChannelStat) Drop() uint64 { return atomic.LoadUint64(&ch.drop) }

// Retransmit returns retransmit counter value
func (ch *ChannelStat) Retransmit() uint64 { return atomic.LoadUint64(&ch.retransmit) }

// incAck increments answers (acknowledgement) counter value
func (ch *ChannelStat) incAck() { atomic.AddUint64(&ch.ack, 1) }

// incAckd increments dropped answers (acknowledgement) counter value
func (ch *ChannelStat) incAckd() { atomic.AddUint64(&ch.ackd, 1) }

// incSent increments send data packets counter value
func (ch *ChannelStat) incSent() {
	atomic.AddUint64(&ch.sent, 1)
	ch.sentSpeed.Add()
}

// incRecv increments received data peckets counter value
func (ch *ChannelStat) incRecv() {
	atomic.AddUint64(&ch.recv, 1)
	ch.recvSpeed.Add()
}

// incDrop increments dropped data peckets counter value
func (ch *ChannelStat) incDrop() { atomic.AddUint64(&ch.drop, 1) }

// incAnswer increments retransmit counter value
func (ch *ChannelStat) incRetransmit() { atomic.AddUint64(&ch.retransmit, 1) }
