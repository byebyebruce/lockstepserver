package server

import (
	"github.com/byebyebruce/lockstepserver/logic"
	"github.com/byebyebruce/lockstepserver/pkg/kcp_server"
	"github.com/byebyebruce/lockstepserver/pkg/network"
	"github.com/byebyebruce/lockstepserver/pkg/packet/pb_packet"
)

// LockStepServer 帧同步服务器
type LockStepServer struct {
	roomMgr   *logic.RoomManager
	udpServer *network.Server
	totalConn int64
}

// New 构造
func New(address string) (*LockStepServer, error) {
	s := &LockStepServer{
		roomMgr: logic.NewRoomManager(),
	}
	networkServer, err := kcp_server.ListenAndServe(address, s, &pb_packet.MsgProtocol{})
	if err != nil {
		return nil, err
	}
	s.udpServer = networkServer
	return s, nil
}

// RoomManager 获取房间管理器
func (r *LockStepServer) RoomManager() *logic.RoomManager {
	return r.roomMgr
}

// Stop 停止服务
func (r *LockStepServer) Stop() {
	r.roomMgr.Stop()
	r.udpServer.Stop()
}
