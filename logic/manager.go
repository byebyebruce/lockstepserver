package logic

import (
	"fmt"
	"sync"

	"github.com/byebyebruce/lockstepserver/logic/room"
)

// RoomManager 房间管理器
type RoomManager struct {
	room map[uint64]*room.Room
	wg   sync.WaitGroup
	rw   sync.RWMutex
}

// NewRoomManager 构造
func NewRoomManager() *RoomManager {
	m := &RoomManager{
		room: make(map[uint64]*room.Room),
	}
	return m
}

// CreateRoom 创建房间
func (m *RoomManager) CreateRoom(id uint64, typeID int32, playerID []uint64, randomSeed int32, logicServer string) (*room.Room, error) {
	m.rw.Lock()
	defer m.rw.Unlock()

	r, ok := m.room[id]
	if ok {
		return nil, fmt.Errorf("room id[%d] exists", id)
	}

	r = room.NewRoom(id, typeID, playerID, randomSeed, logicServer)
	m.room[id] = r

	go func() {
		m.wg.Add(1)
		defer func() {
			m.rw.Lock()
			delete(m.room, id)
			m.rw.Unlock()

			m.wg.Done()
		}()
		r.Run()

	}()

	return r, nil
}

// GetRoom 获得房间
func (m *RoomManager) GetRoom(id uint64) *room.Room {

	m.rw.RLock()
	defer m.rw.RUnlock()

	r, _ := m.room[id]
	return r
}

// RoomNum 获得房间数量
func (m *RoomManager) RoomNum() int {

	m.rw.RLock()
	defer m.rw.RUnlock()

	return len(m.room)
}

// Stop 停止
func (m *RoomManager) Stop() {

	m.rw.Lock()
	for _, v := range m.room {
		v.Stop()
	}
	m.room = make(map[uint64]*room.Room)
	m.rw.Unlock()

	m.wg.Wait()
}
