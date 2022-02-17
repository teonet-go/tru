// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TRU split/combine lage packets module

package tru

// splitPacket split lage packet
func (ch *Channel) splitPacket(data []byte, writeTo func(data []byte, split int) (int, error)) (rid int, err error) {
	var id int
	var pac Packet
	var maxDataLen = func() int {
		if ch.maxDataLen != 0 {
			return ch.maxDataLen
		}
		return pac.MaxDataLen()
	}()
	for i := 0; ; i++ {
		if len(data) <= maxDataLen {
			id, err = writeTo(data, 0)
			if i == 0 {
				rid = id
			}
			break
		}
		rid, err = writeTo(data[:maxDataLen], statusSplit)
		if err != nil {
			return
		}
		data = data[maxDataLen:]
	}
	return
}

// combinePacket combine packet receiver and data structure
type combinePacket struct {
	combine bool
	data    []byte
	first   *Packet
}

// packet combine splitted large packet
func (c *combinePacket) packet(pac *Packet) (retPac *Packet) {
	switch {

	// Start combine
	case !c.combine && pac.status == statusDataNext:
		c.first = pac
		c.data = pac.Data()
		c.combine = true

	// Continue combine
	case c.combine && pac.status == statusDataNext:
		c.data = append(c.data, pac.Data()...)

	// End combine
	case c.combine && pac.status == statusData:
		c.data = append(c.data, pac.Data()...)
		retPac = c.first
		retPac.SetData(c.data[:]).SetStatus(statusData)
		c.clear()

	// Single packet
	case !c.combine && pac.status == statusData:
		retPac = pac
	}

	return
}

// clear combinePacket struct
func (c *combinePacket) clear() {
	c.data = nil
	c.first = nil
	c.combine = false
}
