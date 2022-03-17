package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/byebyebruce/lockstepserver/cmd/example_server/api"
	"github.com/byebyebruce/lockstepserver/pkg/log4gox"
	"github.com/byebyebruce/lockstepserver/server"

	l4g "github.com/alecthomas/log4go"
)

var (
	httpAddress = flag.String("web", ":80", "web listen address")
	udpAddress  = flag.String("udp", ":10086", "udp listen address(':10086' means localhost:10086)")
	debugLog    = flag.Bool("log", true, "debug log")
)

func main() {
	flag.Parse()

	l4g.Close()
	l4g.AddFilter("debug logger", l4g.DEBUG, log4gox.NewColorConsoleLogWriter())

	s, err := server.New(*udpAddress)
	if err != nil {
		panic(err)
	}
	_ = api.NewWebAPI(*httpAddress, s.RoomManager())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	ticker := time.NewTimer(time.Minute)
	defer ticker.Stop()

	l4g.Info("[main] start...")
	// 主循环
QUIT:
	for {
		select {
		case sig := <-sigs:
			l4g.Info("Signal: %s", sig.String())
			break QUIT
		case <-ticker.C:
			// todo
			fmt.Println("room number ", s.RoomManager().RoomNum())
		}
	}
	l4g.Info("[main] quiting...")
	s.Stop()
}
