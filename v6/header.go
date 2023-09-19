package tru

import (
	"bytes"
	"encoding/binary"
)

const (
	pData byte = iota
	pAnswer
	pPing
	pPong
)

type headerPacket struct {
	id    uint32 // packet id
	ptype byte   // 0 - data packet; 1 - answer
}

const headerLen = 5

func (h headerPacket) MarshalBinary() (data []byte, err error) {
	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	binary.Write(buf, le, h.id)
	binary.Write(buf, le, h.ptype)

	data = buf.Bytes()
	return
}

func (h *headerPacket) UnmarshalBinary(data []byte) (err error) {
	buf := bytes.NewReader(data)
	le := binary.LittleEndian

	err = binary.Read(buf, le, &h.id)
	if err != nil {
		return
	}
	err = binary.Read(buf, le, &h.ptype)
	if err != nil {
		return
	}

	return
}
