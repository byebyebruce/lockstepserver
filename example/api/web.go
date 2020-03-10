package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/byebyebruce/lockstepserver/room"
)

func init() {
	http.HandleFunc("/create", HTTPHandleFuncCreate)
	http.HandleFunc("/", HTTPHandleFunc)
}
func HTTPHandleFunc(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()
	if 0 == len(query) {
		t, err := template.New("test").Parse(htmlStr)
		if err != nil {
			w.Write([]byte("error"))
		} else {
			t.Execute(w, nil)
		}
		return
	}
}

func HTTPHandleFuncCreate(w http.ResponseWriter, r *http.Request) {

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

	room, ok := room.CreateRoom(roomID, 0, ps, 0, "test")
	if ok {
		ret = fmt.Sprintf("room.ID=[%d] room.Secret=[%s] room.Time=[%d]", room.ID(), room.SecretKey(), room.TimeStamp())
	} else {
		ret = "failed!"
	}

}
