package game

import (
	"time"

	"github.com/bailu1901/lockstepserver/proto"
	"github.com/bailu1901/lockstepserver/protocol"

	l4g "github.com/alecthomas/log4go"
	"github.com/bailu1901/lockstepserver/network"
	"github.com/golang/protobuf/proto"
)

// GameState 游戏状态
type GameState int

const (
	k_Ready  GameState = 0 //准备阶段
	k_Gaming           = 1 //战斗中阶段
	k_Over             = 2 //结束阶段
	k_Stop             = 3 //停止
)

const (
	MaxReadyTime          int64 = 40     //准备阶段最长时间，如果超过这个时间没人连进来直接关闭游戏
	MaxGameTime           int64 = 60 * 5 //最长游戏时间
	BroadcastOffsetFrames       = 3      //每隔多少帧广播一次
	MaxReconnectFrames          = 60     //重连进来的人每个消息包最多包含多少帧数据
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
	State            GameState
	players          map[uint64]*Player
	logic            *lockstep
	clientFrameCount uint32
	result           map[uint64]uint64

	listener gameListener
}

// NewGame 构造游戏
func NewGame(id uint64, players []uint64, listener gameListener) *Game {
	g := &Game{
		id:        id,
		players:   make(map[uint64]*Player),
		logic:     newLockstep(),
		startTime: time.Now().Unix(),
		listener:  listener,
	}

	for _, v := range players {
		g.players[v] = NewPlayer(v)
	}

	return g
}

// Cleanup 清理游戏
func (g *Game) Cleanup() {
	for _, v := range g.players {
		v.Cleanup()
	}
	g.players = make(map[uint64]*Player)

}

// JoinGame 加入游戏
func (g *Game) JoinGame(id uint64, conn *network.Conn) bool {

	p, ok := g.players[id]
	if !ok {
		l4g.Error("[game(%d)] player[%d] join room failed", g.id, id)
		return false
	}

	if k_Ready != g.State && k_Gaming != g.State {
		l4g.Error("[game(%d)] player[%d] game is over", g.id, id)
		return false
	}

	//把现有的玩家顶掉
	if nil != p.Client {
		p.Client.PutExtraData(nil)
		l4g.Error("[game(%d)] player[%d] replace", g.id, id)
	}

	p.Client = conn
	p.Online = true

	pa := &pb.S2C_JoinRoomMsg{
		Id: proto.Uint64(p.id),
	}
	for _, v := range g.players {
		if p.id == v.id {
			continue
		}
		pa.Others = append(pa.Others, v.id)
	}

	p.SendMessage(protocol.NewPacket(uint8(pb.ID_S2C_JoinRoom), pa))

	//重连进来
	if k_Gaming == g.State {
		g.doReconnect(p)
		l4g.Warn("[game(%d)] doReconnect [%d]", g.id, p.id)
	}

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
func (g *Game) ProcessMsg(id uint64, msg *protocol.Packet) {

	player, ok := g.players[id]
	if !ok {
		l4g.Error("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())
		return
	}
	l4g.Info("[game(%d)] processMsg player[%d] msg=[%d]", g.id, player.id, msg.GetMessageID())

	msgID := pb.ID(msg.GetMessageID())

	switch msgID {
	case pb.ID_C2S_JoinRoom:
	case pb.ID_C2S_Progress:
		m := &pb.C2S_ProgressMsg{}
		msg.UnmarshalPB(m)
		ret := protocol.NewPacket(uint8(pb.ID_S2C_Progress), &pb.S2C_ProgressMsg{

			Id:  proto.Uint64(player.id),
			Pro: m.Pro,
		})
		g.broadcastExclude(ret, player.id)

	case pb.ID_C2S_Heartbeat:
		player.SendMessage(protocol.NewPacket(uint8(pb.ID_S2C_Heartbeat), nil))
	case pb.ID_C2S_Ready:
		g.doReady(player)
	case pb.ID_S2C_InputSkill:
		m := &pb.C2S_InputSkillMsg{}
		msg.UnmarshalPB(m)
		g.pushInput(player, m)
	case pb.ID_C2S_Result:
		// TODO
		g.result[player.id] = player.id
	default:
		l4g.Warn("[game(%d)] processMsg unknown message id[%d]", msgID)
	}

}

// Tick 主逻辑
func (g *Game) Tick(now int64) bool {

	switch g.State {
	case k_Ready:
		delta := now - g.startTime
		if delta > MaxReadyTime {
			g.State = k_Over
			l4g.Error("[game(%d)] game over!! nobody ready", g.id)
		}
		if g.checkReady() {
			g.doStart()
			g.State = k_Gaming
		}
		return true
	case k_Gaming:
		if g.checkOver() {
			g.State = k_Over
			l4g.Info("[game(%d)] game over successfully!!", g.id)
			return true
		}

		delta := now - g.startTime
		if delta > MaxGameTime {
			g.State = k_Over
			l4g.Warn("[game(%d)] game timeout", g.id)
			return true
		}

		g.logic.tick()
		if g.logic.getFrameCount()-g.clientFrameCount >= BroadcastOffsetFrames {
			g.broadcastFrameData()
		}

		return true
	case k_Over:
		g.doGameover()
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

func (g *Game) doReady(p *Player) {

	if k_Ready != g.State {
		return
	}

	if p.Ready == true {
		return
	}

	p.Ready = true

}

func (g *Game) checkReady() bool {
	for _, v := range g.players {
		if !v.Ready {
			return false
		}
	}

	return true
}

func (g *Game) doStart() {

	ret := protocol.NewPacket(uint8(pb.ID_S2C_Ready), &pb.S2C_ReadyMsg{})
	g.broadcast(ret)

	g.clientFrameCount = 0
	g.logic.reset()

	g.listener.OnGameStart(g.id)
}

func (g *Game) doGameover() {
	ret := protocol.NewPacket(uint8(pb.ID_S2C_Result), nil)
	g.broadcast(ret)

	g.listener.OnGameOver(g.id)
}

func (g *Game) pushInput(p *Player, msg *pb.C2S_InputSkillMsg) {

	cmd := &command{
		id:  p.id,
		sid: msg.GetSid(),
		sx:  msg.GetX(),
		sy:  msg.GetY(),
	}

	g.logic.pushCmd(cmd)
}

func (g *Game) doReconnect(p *Player) {

	ret := protocol.NewPacket(uint8(pb.ID_S2C_Ready), &pb.S2C_ReadyMsg{})
	p.SendMessage(ret)

	if g.clientFrameCount <= 0 {
		return
	}

	var i uint32 = 0
	for i < g.clientFrameCount {

		msg := &pb.S2C_FrameMsg{}

		var j uint32 = 0
		for ; j < MaxReconnectFrames && i < g.clientFrameCount; j++ {

			f := &pb.FrameData{}

			f.FrameID = proto.Uint32(i)
			msg.Frames = append(msg.Frames, f)

			frameData := g.logic.getFrame(i)

			if nil != frameData {
				for _, c := range frameData.cmds {
					input := pb.InputData{
						Id:  proto.Uint64(c.id),
						Sid: proto.Int32(c.sid),
						X:   proto.Int32(c.sy),
						Y:   proto.Int32(c.sy),
					}

					f.Input = append(f.Input, &input)
				}
			}

			i++
		}

		p.SendMessage(protocol.NewPacket(uint8(pb.ID_S2C_Frame), msg))

	}

}

func (g *Game) broadcastFrameData() {
	msg := &pb.S2C_FrameMsg{}

	for i := g.clientFrameCount; i < g.logic.getFrameCount(); i++ {
		f := &pb.FrameData{}
		f.FrameID = proto.Uint32(i)
		msg.Frames = append(msg.Frames, f)

		frameData := g.logic.getFrame(i)

		if nil == frameData {
			continue
		}

		for _, c := range frameData.cmds {
			input := pb.InputData{
				Id:  proto.Uint64(c.id),
				Sid: proto.Int32(c.sid),
				X:   proto.Int32(c.sy),
				Y:   proto.Int32(c.sy),
			}

			f.Input = append(f.Input, &input)
		}

	}
	g.clientFrameCount = g.logic.getFrameCount()

	g.broadcast(protocol.NewPacket(uint8(pb.ID_S2C_Frame), msg))

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

func (g *Game) hasOnlinePlayer() bool {

	for _, v := range g.players {
		if v.Online && nil != v.Client {
			return true
		}
	}

	return false
}

func (g *Game) checkOver() bool {
	//只要有人没发结果并且还在线，就不结束
	for _, v := range g.players {
		if !v.Online {
			continue
		}

		if _, ok := g.result[v.id]; !ok {
			return false
		}
	}

	return true
}
