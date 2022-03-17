package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/byebyebruce/lockstepserver/pb"
	"github.com/byebyebruce/lockstepserver/pkg/packet/pb_packet"
	"github.com/golang/protobuf/proto"

	"github.com/xtaci/kcp-go"
)

var (
	addr = flag.String("udp", "127.0.0.1:10086", "connect udp address")
	msg  = flag.String("msg", "PING", "message you want to send")
	room = flag.Uint64("room", 1, "room id")
	id   = flag.Uint64("id", 1, "my id")
)

func main() {

	flag.Parse()

	fmt.Println("addr", *addr, "room", *room, "id", *id)

	ms := &pb_packet.MsgProtocol{}

	c, e := kcp.Dial(*addr)
	if nil != e {
		panic(e)
	}
	defer c.Close()

	// read
	go func() {
		for {
			// c.SetReadDeadline(time.Now().Add(10*time.Second))
			n, e := ms.ReadPacket(c)
			if nil != e {
				fmt.Println("read error:", e.Error())
				return
			}

			// n.Serialize()
			ret := n.(*pb_packet.Packet)
			id := pb.ID(ret.GetMessageID())
			fmt.Println("receive msg ", id.String())
			switch id {
			case pb.ID_MSG_Connect:
				msg := &pb.S2C_ConnectMsg{}
				proto.Unmarshal(ret.GetData(), msg)
				if msg.GetErrorCode() != pb.ERRORCODE_ERR_Ok {
					panic(msg.GetErrorCode())
				}
				fmt.Println(msg)
			case pb.ID_MSG_Frame:
				msg := &pb.S2C_FrameMsg{}
				proto.Unmarshal(ret.GetData(), msg)
				fmt.Println(msg)
			default:

			}
		}
	}()

	// connect
	if _, e := c.Write(pb_packet.NewPacket(uint8(pb.ID_MSG_Connect), &pb.C2S_ConnectMsg{
		PlayerID: proto.Uint64(*id),
		BattleID: proto.Uint64(*room),
	}).Serialize()); nil != e {
		panic(fmt.Sprintf("write error:%s", e.Error()))
	}
	time.Sleep(time.Second)
	// ready
	if _, e := c.Write(pb_packet.NewPacket(uint8(pb.ID_MSG_JoinRoom), nil).Serialize()); nil != e {
		panic(fmt.Sprintf("write error:%s", e.Error()))
	}
	time.Sleep(time.Second)
	// ready
	if _, e := c.Write(pb_packet.NewPacket(uint8(pb.ID_MSG_Ready), nil).Serialize()); nil != e {
		panic(fmt.Sprintf("write error:%s", e.Error()))
	}
	time.Sleep(time.Second)
	// write
	for i := 0; i < 10; i++ {
		p := pb_packet.NewPacket(uint8(pb.ID_MSG_Input), &pb.C2S_InputMsg{
			Sid: proto.Int32(int32(i)),
		})

		if _, e := c.Write(p.Serialize()); nil != e {
			panic(fmt.Sprintf("write error:%s", e.Error()))
		}

		time.Sleep(time.Second)
	}
}
