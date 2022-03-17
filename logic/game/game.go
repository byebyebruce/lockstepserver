package game

import (
	"time"

	"github.com/byebyebruce/lockstepserver/pb"
	"github.com/byebyebruce/lockstepserver/pkg/network"
	"github.com/byebyebruce/lockstepserver/pkg/packet/pb_packet"
	"github.com/golang/protobuf/proto"

	l4g "github.com/alecthomas/log4go"
)

// GameState 游戏状态
type GameState int

const (
	k_Ready  GameState = 0 // 准备阶段
	k_Gaming           = 1 // 战斗中阶段
	k_Over             = 2 // 结束阶段
	k_Stop             = 3 // 停止
)

const (
	MaxReadyTime          int64  = 20            // 准备阶段最长时间，如果超过这个时间没人连进来直接关闭游戏
	MaxGameFrame          uint32 = 30*60*3 + 100 // 每局最大帧数
	BroadcastOffsetFrames        = 3             // 每隔多少帧广播一次
	kMaxFrameDataPerMsg          = 60            // 每个消息包最多包含多少个帧数据
	kBadNetworkThreshold         = 2             // 这个时间段没有收到心跳包认为他网络很差，不再持续给发包(网络层的读写时间设置的比较长，客户端要求的方案)
)

type gameListener interface {
	OnJoinGame(uint64, uint64)
	OnGameStart(uint64)
	OnLeaveGame(uint64, uint64)
	OnGameOver(uint64)
}

// Game 一局游戏
type Game struct {
	id               uint64
	startTime        int64
	randomSeed       int32
	State            GameState
	players          map[uint64]*Player
	logic            *lockstep
	clientFrameCount uint32

	result map[uint64]uint64

	listener gameListener

	dirty bool
}

// NewGame 构造游戏
func NewGame(id uint64, players []uint64, randomSeed int32, listener gameListener) *Game {
	g := &Game{
		id:         id,
		players:    make(map[uint64]*Player),
		logic:      newLockstep(),
		startTime:  time.Now().Unix(),
		randomSeed: randomSeed,
		listener:   listener,
		result:     make(map[uint64]uint64),
	}

	for k, v := range players {
		g.players[v] = NewPlayer(v, int32(k+1))
	}

	return g
}

// JoinGame 加入游戏
func (g *Game) JoinGame(id uint64, conn *network.Conn) bool {

	msg := &pb.S2C_ConnectMsg{
		ErrorCode: pb.ERRORCODE_ERR_Ok.Enum(),
	}

	p, ok := g.players[id]
	if !ok {
		l4g.Error("[game(%d)] player[%d] join room failed", g.id, id)
		return false
	}

	if k_Ready != g.State && k_Gaming != g.State {
		msg.ErrorCode = pb.ERRORCODE_ERR_RoomState.Enum()
		p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), msg))
		l4g.Error("[game(%d)] player[%d] game is over", g.id, id)
		return true
	}

	// 把现有的玩家顶掉
	if nil != p.client {
		// TODO 这里有多线程操作的危险 如果调 p.client.Close() 会把现有刚进来的玩家提调
		p.client.PutExtraData(nil)
		l4g.Error("[game(%d)] player[%d] replace", g.id, id)
	}

	p.Connect(conn)

	p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), msg))

	g.listener.OnJoinGame(g.id, id)

	return true

}

// LeaveGame 离开游戏
func (g *Game) LeaveGame(id uint64) bool {

	p, ok := g.players[id]
	if !ok {
		return false
	}

	p.Cleanup()

	g.listener.OnLeaveGame(g.id, id)

	return true
}

// ProcessMsg 处理消息
func (g *Game) ProcessMsg(id uint64, msg *pb_packet.Packet) {

	player, ok := g.players[id]
	if !ok {
		l4g.Error("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())
		return
	}
	l4g.Info("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())

	msgID := pb.ID(msg.GetMessageID())

	switch msgID {
	case pb.ID_MSG_JoinRoom:
		msg := &pb.S2C_JoinRoomMsg{
			Roomseatid: proto.Int32(player.idx),
			RandomSeed: proto.Int32(g.randomSeed),
		}

		for _, v := range g.players {
			if player.id == v.id {
				continue
			}
			msg.Others = append(msg.Others, v.id)
			msg.Pros = append(msg.Pros, v.loadingProgress)
		}

		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_JoinRoom), msg))

	case pb.ID_MSG_Progress:
		if g.State > k_Ready {
			break
		}
		m := &pb.C2S_ProgressMsg{}
		if err := msg.Unmarshal(m); nil != err {
			l4g.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id, msg.GetMessageID(), err.Error())
			return
		}
		player.loadingProgress = m.GetPro()
		msg := pb_packet.NewPacket(uint8(pb.ID_MSG_Progress), &pb.S2C_ProgressMsg{

			Id:  proto.Uint64(player.id),
			Pro: m.Pro,
		})
		g.broadcastExclude(msg, player.id)

	case pb.ID_MSG_Heartbeat:
		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Heartbeat), nil))
		player.RefreshHeartbeatTime()
	case pb.ID_MSG_Ready:
		if k_Ready == g.State {
			g.doReady(player)
		} else if k_Gaming == g.State {
			g.doReady(player)
			// 重连进来 TODO 对重连进行检查，重连比较耗费
			g.doReconnect(player)
			l4g.Warn("[game(%d)] doReconnect [%d]", g.id, player.id)
		} else {
			l4g.Error("[game(%d)] ID_MSG_Ready player[%d] state error:[%d]", g.id, player.id, g.State)
		}

	case pb.ID_MSG_Input:
		m := &pb.C2S_InputMsg{}
		if err := msg.Unmarshal(m); nil != err {
			l4g.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id, msg.GetMessageID(), err.Error())
			return
		}
		if !g.pushInput(player, m) {
			l4g.Warn("[game(%d)] processMsg player[%d] msg=[%d] pushInput failed", g.id, player.id, msg.GetMessageID())
			break
		}

		// 下一帧强制广播(客户端要求)
		g.dirty = true
	case pb.ID_MSG_Result:
		m := &pb.C2S_ResultMsg{}
		if err := msg.Unmarshal(m); nil != err {
			l4g.Error("[game(%d)] processMsg player[%d] msg=[%d] UnmarshalPB error:[%s]", g.id, player.id, msg.GetMessageID(), err.Error())
			return
		}
		g.result[player.id] = m.GetWinnerID()
		l4g.Info("[game(%d)] ID_MSG_Result player[%d] winner=[%d]", g.id, player.id, m.GetWinnerID())
		player.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Result), nil))
	default:
		l4g.Warn("[game(%d)] processMsg unknown message id[%d]", msgID)
	}

}

// Tick 主逻辑
func (g *Game) Tick(now int64) bool {

	switch g.State {
	case k_Ready:
		delta := now - g.startTime
		if delta < MaxReadyTime {
			if g.checkReady() {
				g.doStart()
				g.State = k_Gaming
			}

		} else {
			if g.getOnlinePlayerCount() > 0 {
				// 大于最大准备时间，只要有在线的，就强制开始
				g.doStart()
				g.State = k_Gaming
				l4g.Warn("[game(%d)] force start game because ready state is timeout ", g.id)
			} else {
				// 全都没连进来，直接结束
				g.State = k_Over
				l4g.Error("[game(%d)] game over!! nobody ready", g.id)
			}
		}

		return true
	case k_Gaming:
		if g.checkOver() {
			g.State = k_Over
			l4g.Info("[game(%d)] game over successfully!!", g.id)
			return true
		}

		if g.isTimeout() {
			g.State = k_Over
			l4g.Warn("[game(%d)] game timeout", g.id)
			return true
		}

		g.logic.tick()
		g.broadcastFrameData()

		return true
	case k_Over:
		g.doGameOver()
		g.State = k_Stop
		l4g.Info("[game(%d)] do game over", g.id)
		return true
	case k_Stop:
		return false
	}

	return false
}

// Result 战斗结果
func (g *Game) Result() map[uint64]uint64 {
	return g.result
}

// Close 关闭游戏
func (g *Game) Close() {
	msg := pb_packet.NewPacket(uint8(pb.ID_MSG_Close), nil)
	g.broadcast(msg)
}

// Cleanup 清理游戏
func (g *Game) Cleanup() {
	for _, v := range g.players {
		v.Cleanup()
	}
	g.players = make(map[uint64]*Player)

}

func (g *Game) doReady(p *Player) {

	if p.isReady == true {
		return
	}

	p.isReady = true

	msg := pb_packet.NewPacket(uint8(pb.ID_MSG_Ready), nil)
	p.SendMessage(msg)
}

func (g *Game) checkReady() bool {
	for _, v := range g.players {
		if !v.isReady {
			return false
		}
	}

	return true
}

func (g *Game) doStart() {

	g.clientFrameCount = 0
	g.logic.reset()
	for _, v := range g.players {
		v.isReady = true
		v.loadingProgress = 100
	}
	g.startTime = time.Now().Unix()
	msg := &pb.S2C_StartMsg{
		TimeStamp: proto.Int64(g.startTime),
	}
	ret := pb_packet.NewPacket(uint8(pb.ID_MSG_Start), msg)

	g.broadcast(ret)

	g.listener.OnGameStart(g.id)
}

func (g *Game) doGameOver() {

	g.listener.OnGameOver(g.id)
}

func (g *Game) pushInput(p *Player, msg *pb.C2S_InputMsg) bool {

	cmd := &pb.InputData{
		Id:         proto.Uint64(p.id),
		Sid:        proto.Int32(msg.GetSid()),
		X:          proto.Int32(msg.GetX()),
		Y:          proto.Int32(msg.GetY()),
		Roomseatid: proto.Int32(p.idx),
	}

	return g.logic.pushCmd(cmd)
}

func (g *Game) doReconnect(p *Player) {

	msg := &pb.S2C_StartMsg{
		TimeStamp: proto.Int64(g.startTime),
	}
	ret := pb_packet.NewPacket(uint8(pb.ID_MSG_Start), msg)
	p.SendMessage(ret)

	framesCount := g.clientFrameCount
	var i uint32 = 0
	c := 0
	frameMsg := &pb.S2C_FrameMsg{}

	for ; i < framesCount; i++ {

		frameData := g.logic.getFrame(i)
		if nil == frameData && i != (framesCount-1) {
			continue
		}

		f := &pb.FrameData{
			FrameID: proto.Uint32(i),
		}

		if nil != frameData {
			f.Input = frameData.cmds
		}
		frameMsg.Frames = append(frameMsg.Frames, f)
		c++

		if c >= kMaxFrameDataPerMsg || i == (framesCount-1) {
			p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Frame), frameMsg))
			c = 0
			frameMsg = &pb.S2C_FrameMsg{}
		}
	}

	p.SetSendFrameCount(g.clientFrameCount)

}

func (g *Game) broadcastFrameData() {

	framesCount := g.logic.getFrameCount()

	if !g.dirty && framesCount-g.clientFrameCount < BroadcastOffsetFrames {
		return
	}

	defer func() {
		g.dirty = false
		g.clientFrameCount = framesCount
	}()

	/*
		msg := &pb.S2C_FrameMsg{}

		for i := g.clientFrameCount; i < g.logic.getFrameCount(); i++ {
			frameData := g.logic.getFrame(i)

			if nil == frameData && i != (g.logic.getFrameCount()-1) {
				continue
			}

			f := &pb.FrameData{}
			f.FrameID = proto.Uint32(i)
			msg.Frames = append(msg.Frames, f)

			if nil != frameData {
				f.Input = frameData.cmds
			}

		}
		if len(msg.Frames) > 0 {
			g.broadcast(pb_packet.NewPacket(uint8(pb.ID_MSG_Frame), msg))
		}
	*/
	now := time.Now().Unix()

	for _, p := range g.players {

		// 掉线的
		if !p.IsOnline() {
			continue
		}

		if !p.isReady {
			continue
		}

		// 网络不好的
		if now-p.GetLastHeartbeatTime() >= kBadNetworkThreshold {
			continue
		}

		// 获得这个玩家已经发到哪一帧
		i := p.GetSendFrameCount()
		c := 0
		msg := &pb.S2C_FrameMsg{}
		for ; i < framesCount; i++ {
			frameData := g.logic.getFrame(i)
			if nil == frameData && i != (framesCount-1) {
				continue
			}

			f := &pb.FrameData{
				FrameID: proto.Uint32(i),
			}

			if nil != frameData {
				f.Input = frameData.cmds
			}
			msg.Frames = append(msg.Frames, f)
			c++

			// 如果是最后一帧或者达到这个消息包能装下的最大帧数，就发送
			if i == (framesCount-1) || c >= kMaxFrameDataPerMsg {
				p.SendMessage(pb_packet.NewPacket(uint8(pb.ID_MSG_Frame), msg))
				c = 0
				msg = &pb.S2C_FrameMsg{}
			}

		}

		p.SetSendFrameCount(framesCount)

	}

}

func (g *Game) broadcast(msg network.Packet) {
	for _, v := range g.players {
		v.SendMessage(msg)
	}
}

func (g *Game) broadcastExclude(msg network.Packet, id uint64) {
	for _, v := range g.players {
		if v.id == id {
			continue
		}
		v.SendMessage(msg)
	}
}

func (g *Game) getPlayer(id uint64) *Player {

	return g.players[id]
}

func (g *Game) getPlayerCount() int {

	return len(g.players)
}

func (g *Game) getOnlinePlayerCount() int {

	i := 0
	for _, v := range g.players {
		if v.IsOnline() {
			i++
		}
	}

	return i
}

func (g *Game) checkOver() bool {
	// 只要有人没发结果并且还在线，就不结束
	for _, v := range g.players {
		if !v.isOnline {
			continue
		}

		if _, ok := g.result[v.id]; !ok {
			return false
		}
	}

	return true
}

func (g *Game) isTimeout() bool {
	return g.logic.getFrameCount() > MaxGameFrame

	return true
}
