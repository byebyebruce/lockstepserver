package main

//-----------------------------------
//File:main.go
//Date:2018年8月23日
//Desc:帧同步战斗服务器
//Auth:Bruce
//-----------------------------------

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	l4g "github.com/alecthomas/log4go"
	"github.com/byebyebruce/lockstepserver/example/api"
	"github.com/byebyebruce/lockstepserver/kcp_server"
	"github.com/byebyebruce/lockstepserver/protocol"
	"github.com/byebyebruce/lockstepserver/room"
	"github.com/byebyebruce/lockstepserver/router"
	"github.com/byebyebruce/lockstepserver/util"
)

var (
	nodeId     = flag.Uint64("id", 0, "id")
	gWeb       = flag.String("web", ":10002", "web listen address")
	outAddress = flag.String("out", ":10086", "out listen address(':10086' means localhost:10086)")
	m          *room.RoomManager
)

//Init 初始化
func LoadConfig() bool {
	return true
}

//Init 初始化
func Init() bool {
	go func() {
		e := http.ListenAndServe(*gWeb, nil)
		if nil != e {
			panic(e)
		}
	}()
	l4g.Info("[main] http.ListenAndServe port=[%s]", *gWeb)

	return true
}

//Run 运行
func Run() {

	defer func() {
		//clear
		time.Sleep(time.Millisecond * 100)
		l4g.Warn("[main] pvp %d quit", *nodeId)
		l4g.Global.Close()
	}()

	//address := util.GetLocalIP()
	//udp server
	networkServer, err := kcp_server.ListenAndServe(*outAddress, router.New(m), &protocol.MsgProtocol{})
	if nil != err {
		panic(err)
	}
	l4g.Info("[main] kcp.Listen addr=[%s]", *outAddress)
	defer networkServer.Stop()

	m = room.NewRoomManager()
	defer m.Stop()

	_ = api.NewHttpApi(m)

	//主循环定时器
	ticker := time.NewTimer(time.Second)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	l4g.Warn("[main] %d running...", *nodeId)
	//主循环
QUIT:
	for {
		select {
		case sig := <-sigs:
			l4g.Info("Signal: %s", sig.String())
			if sig == syscall.SIGHUP {

			} else {
				break QUIT
			}
		case <-ticker.C:

		}

	}

	l4g.Info("[main] pvp %d quiting...", *nodeId)
}

func main() {

	showIP := false
	flag.BoolVar(&showIP, "ip", false, "show ip info")
	flag.Parse()
	if showIP {
		fmt.Println("GetOutboundIP", util.GetOutboundIP())
		fmt.Println("GetLocalIP", util.GetLocalIP())
		fmt.Println("GetExternalIP", util.GetExternalIP())
		os.Exit(0)
	}

	if LoadConfig() && Init() {
		Run()
	} else {
		fmt.Printf("[main] launch fail")
	}

}
