package room

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/byebyebruce/lockstepserver/logic/game"
	"github.com/byebyebruce/lockstepserver/pkg/network"
	"github.com/byebyebruce/lockstepserver/pkg/packet/pb_packet"

	l4g "github.com/alecthomas/log4go"
)

const (
	Frequency   = 30                      // 每分钟心跳频率
	TickTimer   = time.Second / Frequency // 心跳Timer
	TimeoutTime = time.Minute * 5         // 超时时间
)

type packet struct {
	id  uint64
	msg network.Packet
}

// Room 战斗房间
type Room struct {
	wg sync.WaitGroup

	roomID      uint64
	players     []uint64
	typeID      int32
	closeFlag   int32
	timeStamp   int64
	secretKey   string
	logicServer string

	exitChan chan struct{}
	msgQ     chan *packet
	inChan   chan *network.Conn
	outChan  chan *network.Conn

	game *game.Game
}

// NewRoom 构造
func NewRoom(id uint64, typeID int32, players []uint64, randomSeed int32, logicServer string) *Room {
	r := &Room{
		roomID:      id,
		players:     players,
		typeID:      typeID,
		exitChan:    make(chan struct{}),
		msgQ:        make(chan *packet, 2048),
		outChan:     make(chan *network.Conn, 8),
		inChan:      make(chan *network.Conn, 8),
		timeStamp:   time.Now().Unix(),
		logicServer: logicServer,
		secretKey:   "test_room",
	}

	r.game = game.NewGame(id, players, randomSeed, r)

	return r
}

// ID room ID
func (r *Room) ID() uint64 {
	return r.roomID
}

// SecretKey secret key
func (r *Room) SecretKey() string {
	return r.secretKey
}

// TimeStamp time stamp
func (r *Room) TimeStamp() int64 {
	return r.timeStamp
}

// IsOver 是否已经结束
func (r *Room) IsOver() bool {
	return atomic.LoadInt32(&r.closeFlag) != 0
}

// HasPlayer 是否有这个player
func (r *Room) HasPlayer(id uint64) bool {
	for _, v := range r.players {
		if v == id {
			return true
		}
	}

	return false
}

func (r *Room) OnJoinGame(id, pid uint64) {
	l4g.Warn("[room(%d)] onJoinGame %d", id, pid)
}
func (r *Room) OnGameStart(id uint64) {
	l4g.Warn("[room(%d)] onGameStart", id)
}

func (r *Room) OnLeaveGame(id, pid uint64) {
	l4g.Warn("[room(%d)] onLeaveGame %d", id, pid)
}
func (r *Room) OnGameOver(id uint64) {
	atomic.StoreInt32(&r.closeFlag, 1)

	l4g.Warn("[room(%d)] onGameOver", id)

	r.wg.Add(1)

	go func() {
		defer r.wg.Done()
		// TODO
		// http result
	}()

}

// OnConnect network.Conn callback
func (r *Room) OnConnect(conn *network.Conn) bool {

	conn.SetCallback(r) // SetCallback只能在OnConnect里调
	r.inChan <- conn
	l4g.Warn("[room(%d)] OnConnect %d", r.roomID, conn.GetExtraData().(uint64))

	return true
}

// OnMessage network.Conn callback
func (r *Room) OnMessage(conn *network.Conn, msg network.Packet) bool {

	id, ok := conn.GetExtraData().(uint64)
	if !ok {
		l4g.Error("[room] OnMessage error conn don't have id")
		return false
	}

	p := &packet{
		id:  id,
		msg: msg,
	}
	r.msgQ <- p

	return true
}

// OnClose network.Conn callback
func (r *Room) OnClose(conn *network.Conn) {
	r.outChan <- conn
	if id, ok := conn.GetExtraData().(uint64); ok {
		l4g.Warn("[room(%d)] OnClose %d", r.roomID, id)
	} else {
		l4g.Warn("[room(%d)] OnClose no id", r.roomID)
	}

}

// Run 主循环
func (r *Room) Run() {
	r.wg.Add(1)
	defer r.wg.Done()
	defer func() {
		/*
			err := recover()
			if nil != err {
				l4g.Error("[room(%d)] Run error:%+v", r.roomID, err)
			}*/
		r.game.Cleanup()
		l4g.Warn("[room(%d)] quit! total time=[%d]", r.roomID, time.Now().Unix()-r.timeStamp)
	}()

	// 心跳
	tickerTick := time.NewTicker(TickTimer)
	defer tickerTick.Stop()

	// 超时timer
	timeoutTimer := time.NewTimer(TimeoutTime)

	l4g.Info("[room(%d)] running...", r.roomID)

LOOP:
	for {
		select {
		case <-r.exitChan:
			l4g.Error("[room(%d)] force exit", r.roomID)
			return
		case <-timeoutTimer.C:
			l4g.Error("[room(%d)] time out", r.roomID)
			break LOOP
		case msg := <-r.msgQ:
			r.game.ProcessMsg(msg.id, msg.msg.(*pb_packet.Packet))
		case <-tickerTick.C:
			if !r.game.Tick(time.Now().Unix()) {
				l4g.Info("[room(%d)] tick over", r.roomID)
				break LOOP
			}
		case c := <-r.inChan:
			id, ok := c.GetExtraData().(uint64)
			if ok {
				if r.game.JoinGame(id, c) {
					l4g.Info("[room(%d)] player[%d] join room ok", r.roomID, id)
				} else {
					l4g.Error("[room(%d)] player[%d] join room failed", r.roomID, id)
					c.Close()
				}
			} else {
				c.Close()
				l4g.Error("[room(%d)] inChan don't have id", r.roomID)
			}

		case c := <-r.outChan:
			if id, ok := c.GetExtraData().(uint64); ok {
				r.game.LeaveGame(id)
			} else {
				c.Close()
				l4g.Error("[room(%d)] outChan don't have id", r.roomID)
			}
		}
	}

	r.game.Close()

	for i := 3; i > 0; i-- {
		<-time.After(time.Second)
		l4g.Info("[room(%d)] quiting %d...", r.roomID, i)
	}
}

// Stop 强制关闭
func (r *Room) Stop() {
	close(r.exitChan)
	r.wg.Wait()
}
