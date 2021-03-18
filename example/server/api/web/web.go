package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/byebyebruce/lockstepserver/room"
)

// WebAPI http api
type WebAPI struct {
	m *room.RoomManager
}

// NewWebAPI 构造
func NewWebAPI(m *room.RoomManager) *WebAPI {
	r := &WebAPI{
		m: m,
	}

	http.HandleFunc("/", r.index)
	http.HandleFunc("/create", r.createRoom)

	return r
}

func (h *WebAPI) index(w http.ResponseWriter, r *http.Request) {

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

const htmlStr = `<html>  
    <head>  
        <title>command</title>  
    </head>  
    <body>  
        <style type="text/css">
            #all {
                color: #000000;
                background: #ececec;
                width: 300px;
                height: 200px;
            }

            div {
                line-height: 40px;
                text-align: right;
            }

            input {
                width: 400px;
                height: 20px;
            }

            select {
                padding: 5px 82px;
            }
        </style>
       

    
        <label for="API">Room:</label>
        <p>
            <!-- <input type="text" name="API" id="api" value="help" rows="100"> -->
            <textarea id="room" name="API" rows="2" cols="30">1</textarea>
			
            
        </p>

		<label for="API">Member:</label>
		<p>
			<textarea id="member" name="API" rows="2" cols="30">1,2</textarea>
		</p>

        <p>
            <button id="btn" style="height:50px;width:100px">Create Room</button>
        </p>

        <label for="Return">Return:</label>
        <p>
            <textarea id="return" name="summary" rows="50" cols="120"></textarea>
        </p>
        
    </body>  
    <script src="http://apps.bdimg.com/libs/jquery/2.1.4/jquery.min.js"></script>

    <script type="text/javascript">
        

        $("#btn").on("click", function () {
            var param = $("#room").val()
			var param1 = $("#member").val()
			

            var url1 = window.location.href +'/create?room='+param+'&member='+param1

            document.getElementById("return").value = "waiting result..."
            
            console.log(url1)
            $.ajax({
                url: url1,
                type: "GET",
				contentType: "application/json; charset=utf-8",
				//dataType: "json",

                //data: param,

                success: function (res) {
                    document.getElementById("return").value = res;
                },

                error: function(xhr,textStatus){
                    document.getElementById("return").value = "error, state = " + textStatus
                }
            })
            
        })

    </script>
</html>`
