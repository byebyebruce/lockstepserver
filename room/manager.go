package room

import (
	"sync"
)

var (
	room map[uint64]*Room
	wg   sync.WaitGroup
	rw   sync.RWMutex
)

func init() {
	room = make(map[uint64]*Room)
}

func CreateRoom(id uint64, typeID int32, playerID []uint64, randomSeed int32, logicServer string) (*Room, bool) {
	rw.Lock()
	defer rw.Unlock()

	r, ok := room[id]
	if ok {
		return nil, false
	}

	r = NewRoom(id, typeID, playerID, randomSeed, logicServer)
	room[id] = r

	go func() {
		wg.Add(1)
		defer func() {
			rw.Lock()
			delete(room, id)
			rw.Unlock()

			wg.Done()
		}()
		r.Run()

	}()

	return r, true
}

func GetRoom(id uint64) *Room {

	rw.RLock()
	defer rw.RUnlock()

	r, _ := room[id]
	return r
}
func Stop() {

	rw.Lock()
	for _, v := range room {
		v.Stop()
	}
	room = make(map[uint64]*Room)
	rw.Unlock()

	wg.Wait()
}
