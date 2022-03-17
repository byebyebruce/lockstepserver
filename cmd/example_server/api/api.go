package api

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

	"github.com/byebyebruce/lockstepserver/logic"
)

//go:embed index.html
var index string

// WebAPI http api
type WebAPI struct {
	m *logic.RoomManager
}

// NewWebAPI 构造
func NewWebAPI(addr string, m *logic.RoomManager) *WebAPI {
	r := &WebAPI{
		m: m,
	}

	http.HandleFunc("/", r.index)
	http.HandleFunc("/create", r.createRoom)

	go func() {
		fmt.Println("web api listen on", addr)
		e := http.ListenAndServe(addr, nil)
		if nil != e {
			panic(e)
		}
	}()

	return r
}

func (h *WebAPI) index(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()
	if 0 == len(query) {
		t, err := template.New("test").Parse(index)
		if err != nil {
			w.Write([]byte("error"))
		} else {
			t.Execute(w, nil)
		}
		return
	}
}

func (h *WebAPI) createRoom(w http.ResponseWriter, r *http.Request) {

	ret := "error"

	defer func() {
		w.Write([]byte(ret))
	}()

	query := r.URL.Query()

	roomStr := query.Get("room")
	roomID, _ := strconv.ParseUint(roomStr, 10, 64)

	ps := make([]uint64, 0, 10)

	members := query.Get("member")
	if len(members) > 0 {

		a := strings.Split(members, ",")

		for _, v := range a {
			id, _ := strconv.ParseUint(v, 10, 64)
			ps = append(ps, id)
		}

	}

	room, err := h.m.CreateRoom(roomID, 0, ps, 0, "test")
	if nil != err {
		ret = err.Error()
	} else {
		ret = fmt.Sprintf("room.ID=[%d] room.Secret=[%s] room.Time=[%d], room.Member=[%v]", room.ID(), room.SecretKey(), room.TimeStamp(), members)
	}

}
