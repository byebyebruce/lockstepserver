package game

import (
	"github.com/bailu1901/lockstepserver/network"
)

type Player struct {
	id     uint64
	Client *network.Conn
	Ready  bool
	Online bool
}

func NewPlayer(id uint64) *Player {
	p := &Player{
		id: id,
	}

	return p
}

func (p *Player) SendMessage(msg network.Packet) {

	if !p.Online {
		return
	}

	if nil != p.Client {
		p.Client.AsyncWritePacket(msg, 0)
	}
}

func (p *Player) Cleanup() {

	if nil != p.Client {
		p.Client.Close()
	}
	p.Client = nil
	p.Ready = false
	p.Online = false

}
