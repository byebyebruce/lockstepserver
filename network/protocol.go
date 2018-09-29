package network

import (
	"encoding/binary"
	"errors"
	"io"
)

type Packet interface {
	Serialize() []byte
}

type Protocol interface {
	ReadPacket(conn io.Reader) (Packet, error)
}

type DefaultPacket struct {
	buff []byte
}

func (this *DefaultPacket) Serialize() []byte {
	return this.buff
}

func (this *DefaultPacket) GetBody() []byte {
	return this.buff[4:]
}

func NewDefaultPacket(buff []byte) *DefaultPacket {
	p := &DefaultPacket{}

	p.buff = make([]byte, 4+len(buff))
	binary.BigEndian.PutUint32(p.buff[0:4], uint32(len(buff)))
	copy(p.buff[4:], buff)

	return p
}

type DefaultProtocol struct {
}

func (this *DefaultProtocol) ReadPacket(r io.Reader) (Packet, error) {
	var (
		lengthBytes []byte = make([]byte, 4)
		length      uint32
	)

	// read length
	if _, err := io.ReadFull(r, lengthBytes); err != nil {
		return nil, err
	}
	if length = binary.BigEndian.Uint32(lengthBytes); length > 1024 {
		return nil, errors.New("the size of packet is larger than the limit")
	}

	buff := make([]byte, length)

	// read body ( buff = lengthBytes + body )
	if _, err := io.ReadFull(r, buff); err != nil {
		return nil, err
	}

	return NewDefaultPacket(buff), nil
}
