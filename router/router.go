package router

import (
	"sync/atomic"
	"time"

	"github.com/byebyebruce/lockstepserver/network"
	"github.com/byebyebruce/lockstepserver/pb"
	"github.com/byebyebruce/lockstepserver/protocol"
	"github.com/byebyebruce/lockstepserver/room"

	l4g "github.com/alecthomas/log4go"
)

// TODO
func verifyToken(secret string) string {
	return secret
}

// Router 消息路由器
type Router struct {
	m         *room.RoomManager
	totalConn uint64
}

// New 构造
func New(m *room.RoomManager) *Router {
	return &Router{
		m: m,
	}
}

// OnConnect 链接进来
func (r *Router) OnConnect(conn *network.Conn) bool {

	id := atomic.AddUint64(&r.totalConn, 1)
	l4g.Debug("[router] OnConnect [%s] totalConn=%d", conn.GetRawConn().RemoteAddr().String(), id)
	return true
}

// OnMessage 消息处理
func (r *Router) OnMessage(conn *network.Conn, p network.Packet) bool {

	msg := p.(*protocol.Packet)

	l4g.Info("[router] OnMessage [%s] msg=[%d] len=[%d]", conn.GetRawConn().RemoteAddr().String(), msg.GetMessageID(), len(msg.GetData()))

	switch pb.ID(msg.GetMessageID()) {
	case pb.ID_MSG_Connect:

		rec := &pb.C2S_ConnectMsg{}
		if err := msg.UnmarshalPB(rec); nil != err {
			l4g.Error("[router] msg.UnmarshalPB error=[%s]", err.Error())
			return false
		}

		//player id
		playerID := rec.GetPlayerID()
		//room id
		roomID := rec.GetBattleID()
		//token
		token := rec.GetToken()

		ret := &pb.S2C_ConnectMsg{
			ErrorCode: pb.ERRORCODE_ERR_Ok.Enum(),
		}

		room := r.m.GetRoom(roomID)
		if nil == room {
			ret.ErrorCode = pb.ERRORCODE_ERR_NoRoom.Enum()
			conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			l4g.Error("[router] no room player=[%d] room=[%d] token=[%s]", playerID, roomID, token)
			return true
		}

		if room.IsOver() {
			ret.ErrorCode = pb.ERRORCODE_ERR_RoomState.Enum()
			conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			l4g.Error("[router] room is over player=[%d] room==[%d] token=[%s]", playerID, roomID, token)
			return true
		}

		if !room.HasPlayer(playerID) {
			ret.ErrorCode = pb.ERRORCODE_ERR_NoPlayer.Enum()
			conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			l4g.Error("[router] !room.HasPlayer(playerID) player=[%d] room==[%d] token=[%s]", playerID, roomID, token)
			return true
		}

		// 验证token
		if token != verifyToken(token) {
			ret.ErrorCode = pb.ERRORCODE_ERR_Token.Enum()
			conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
			l4g.Error("[router] verifyToken failed player=[%d] room==[%d] token=[%s]", playerID, roomID, token)
			return true
		}

		conn.PutExtraData(playerID)

		//这里只是先给加上身份标识，不能直接返回Connect成功，又后面Game返回
		//conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Connect), ret), time.Millisecond)
		return room.OnConnect(conn)

	case pb.ID_MSG_Heartbeat:
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_Heartbeat), nil), time.Millisecond)
		return true

	case pb.ID_MSG_END:
		// 正式版不会提供这个消息
		conn.AsyncWritePacket(protocol.NewPacket(uint8(pb.ID_MSG_END), msg.GetData()), time.Millisecond)
		return true
	}

	return false

}

// OnClose 链接断开
func (r *Router) OnClose(conn *network.Conn) {
	id := atomic.LoadUint64(&r.totalConn) - 1
	atomic.StoreUint64(&r.totalConn, id)

	l4g.Info("[router] OnClose: total=%d", id)
}
