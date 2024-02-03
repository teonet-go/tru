package tru

import "sync/atomic"

// ChannelStat contains statistics data (atomic vars) and methods to work with it
type ChannelStat struct {
	sent       uint64 // Number of data packets sent
	ack        uint64 // Number of packets answer (acknowledgement) received
	ackd       uint64 // Number of dropped packets answer (acknowledgement)
	retransmit uint64 // Number of packets data retransmited
	retrprev   uint64 // Previouse retransmits value
	recv       uint64 // Number of data packets received
	drop       uint64 // Number of dropped received data packets
}

// Ack returns answer (acknowledgement) counter value
func (ch *ChannelStat) Ack() uint64 { return atomic.LoadUint64(&ch.ack) }

// ackd returns dropped answer (acknowledgement) counter value
func (ch *ChannelStat) Ackd() uint64 { return atomic.LoadUint64(&ch.ackd) }

// Sent returns data packet sent counter value
func (ch *ChannelStat) Sent() uint64 { return atomic.LoadUint64(&ch.sent) }

// Recv returns data packet received counter value
func (ch *ChannelStat) Recv() uint64 { return atomic.LoadUint64(&ch.recv) }

// Drop returns number of droppet received data packets (duplicate data packets)
func (ch *ChannelStat) Drop() uint64 { return atomic.LoadUint64(&ch.drop) }

// Retransmit returns retransmit counter value
func (ch *ChannelStat) Retransmit() uint64 { return atomic.LoadUint64(&ch.retransmit) }

// incAck increments answers (acknowledgement) counter value
func (ch *ChannelStat) incAck() { atomic.AddUint64(&ch.ack, 1) }

// incAckd increments dropped answers (acknowledgement) counter value
func (ch *ChannelStat) incAckd() { atomic.AddUint64(&ch.ackd, 1) }

// incSent increments send data peckets counter value
func (ch *ChannelStat) incSent() { atomic.AddUint64(&ch.sent, 1) }

// incRecv increments received data peckets counter value
func (ch *ChannelStat) incRecv() { atomic.AddUint64(&ch.recv, 1) }

// incDrop increments dropped data peckets counter value
func (ch *ChannelStat) incDrop() { atomic.AddUint64(&ch.drop, 1) }

// incAnswer increments retransmit counter value
func (ch *ChannelStat) incRetransmit() { atomic.AddUint64(&ch.retransmit, 1) }