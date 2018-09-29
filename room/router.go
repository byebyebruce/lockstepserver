package room

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bailu1901/lockstepserver/network"
	"github.com/bailu1901/lockstepserver/proto"
	"github.com/bailu1901/lockstepserver/protocol"

	"fmt"

	l4g "github.com/alecthomas/log4go"
)

type Router struct {
	sessiondID uint64
	lost       uint64
}

func (m *Router) OnConnect(conn *network.Conn) bool {

	id := atomic.AddUint64(&m.sessiondID, 1)
	l4g.Debug("OnConnect [%s] %d", conn.GetRawConn().RemoteAddr().String(), id)
	return true
}

func (m *Router) OnMessage(conn *network.Conn, p network.Packet) bool {

	msg := p.(*protocol.Packet)

	l4g.Info("OnMessage [%s] msg=[%d] len=[%d]", conn.GetRawConn().RemoteAddr().String(), msg.GetMessageID(), len(msg.GetData()))

	switch pb.ID(msg.GetMessageID()) {
	case pb.ID_C2S_Connect:
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_S2C_Connect), nil), time.Millisecond)

		// TODO
		rec := &pb.C2S_ConnectMsg{}
		if nil != msg.UnmarshalPB(rec) {
			return false
		}

		token := rec.GetToken()
		l4g.Trace("ID_C2S_Connect token=[%s]", token)

		a := strings.Split(token, ",")
		if 2 != len(a) {
			return false
		}

		rId, _ := strconv.ParseUint(a[0], 10, 64)
		id, _ := strconv.ParseUint(a[1], 10, 64)

		r := GetRoom(rId)
		if nil == r {
			return false
		}

		//id := atomic.AddUint64(&m.sessiondID, 1)

		conn.PutExtraData(id)
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_S2C_Connect), nil), time.Millisecond)
		return r.OnConnect(conn)

	case pb.ID_C2S_Heartbeat:
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_S2C_Heartbeat), nil), time.Millisecond)
		return true
	case pb.ID_MSG_END: //test
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_END), msg.GetData()), time.Millisecond)
		return true
	default:
		return false
	}

	return false

}

func (m *Router) OnClose(conn *network.Conn) {
	id := atomic.AddUint64(&m.lost, 1)
	fmt.Println("OnClose:", id)

	//atomic.AddUint64(&m.lost, 1)
}
