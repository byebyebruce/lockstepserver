package pb_packet

import (
	"encoding/binary"
	"errors"
	"io"

	l4g "github.com/alecthomas/log4go"
	"github.com/byebyebruce/lockstepserver/pkg/network"
	"github.com/golang/protobuf/proto"
)

const (
	DataLen      = 2
	MessageIDLen = 1

	MinPacketLen = DataLen + MessageIDLen
	MaxPacketLen = (2 << 8) * DataLen
	MaxMessageID = (2 << 8) * MessageIDLen
)

/*

s->c

|--totalDataLen(uint16)--|--msgIDLen(uint8)--|--------------data--------------|
|-------------2----------|---------1---------|---------(totalDataLen-2-1)-----|

*/

// Packet 服务端发往客户端的消息
type Packet struct {
	id   uint8
	data []byte
}

func (p *Packet) GetMessageID() uint8 {
	return p.id
}

func (p *Packet) GetData() []byte {
	return p.data
}

func (p *Packet) Serialize() []byte {
	buff := make([]byte, MinPacketLen, MinPacketLen)

	dataLen := len(p.data)
	binary.BigEndian.PutUint16(buff, uint16(dataLen))

	buff[DataLen] = p.id
	return append(buff, p.data...)
}

func (p *Packet) Unmarshal(m interface{}) error {
	return proto.Unmarshal(p.data, m.(proto.Message))
}

func NewPacket(id uint8, msg interface{}) *Packet {

	p := &Packet{
		id: id,
	}

	switch v := msg.(type) {
	case []byte:
		p.data = v
	case proto.Message:
		if mdata, err := proto.Marshal(v); err == nil {
			p.data = mdata
		} else {
			l4g.Error("[NewPacket] proto marshal msg: %d error: %v",
				id, err)
			return nil
		}
	case nil:
	default:
		l4g.Error("[NewPacket] error msg type msg: %d", id)
		return nil
	}

	return p
}

type MsgProtocol struct {
}

func (p *MsgProtocol) ReadPacket(r io.Reader) (network.Packet, error) /*Packet*/ {

	buff := make([]byte, MinPacketLen, MinPacketLen)

	// data length
	if _, err := io.ReadFull(r, buff); err != nil {
		return nil, err
	}
	dataLen := binary.BigEndian.Uint16(buff)

	if dataLen > MaxPacketLen {
		return nil, errors.New("data max")
	}

	// id
	msg := &Packet{
		id: buff[DataLen],
	}

	// data
	if dataLen > 0 {
		msg.data = make([]byte, dataLen, dataLen)
		if _, err := io.ReadFull(r, msg.data); err != nil {
			return nil, err
		}
	}

	return msg, nil
}
