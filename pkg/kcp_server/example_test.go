package kcp_server

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/byebyebruce/lockstepserver/pkg/network"

	"github.com/xtaci/kcp-go"
)

const (
	latency = time.Millisecond * 10
)

type testCallback struct {
	numConn   uint32
	numMsg    uint32
	numDiscon uint32
}

func (t *testCallback) OnMessage(conn *network.Conn, msg network.Packet) bool {

	atomic.AddUint32(&t.numMsg, 1)

	// fmt.Println("OnMessage", conn.GetExtraData(), string(msg.(*network.DefaultPacket).GetBody()))
	conn.AsyncWritePacket(network.NewDefaultPacket([]byte("pong")), time.Second*1)
	return true
}

func (t *testCallback) OnConnect(conn *network.Conn) bool {
	id := atomic.AddUint32(&t.numConn, 1)
	conn.PutExtraData(id)
	// fmt.Println("OnConnect", conn.GetExtraData())
	return true
}

func (t *testCallback) OnClose(conn *network.Conn) {
	atomic.AddUint32(&t.numDiscon, 1)

	// fmt.Println("OnDisconnect", conn.GetExtraData())
}

func Test_KCPServer(t *testing.T) {

	l, err := kcp.Listen(":10086")
	if nil != err {
		panic(err)
	}

	config := &network.Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
		ConnReadTimeout:        latency,
		ConnWriteTimeout:       latency,
	}

	callback := &testCallback{}
	server := network.NewServer(config, callback, &network.DefaultProtocol{})

	go server.Start(l, func(conn net.Conn, i *network.Server) *network.Conn {
		kcpConn := conn.(*kcp.UDPSession)
		kcpConn.SetNoDelay(1, 10, 2, 1)
		kcpConn.SetStreamMode(true)
		kcpConn.SetWindowSize(4096, 4096)
		kcpConn.SetReadBuffer(4 * 1024 * 1024)
		kcpConn.SetWriteBuffer(4 * 1024 * 1024)
		kcpConn.SetACKNoDelay(true)

		return network.NewConn(conn, server)
	})
	defer server.Stop()

	time.Sleep(time.Second)

	wg := sync.WaitGroup{}
	const max_con = 100
	for i := 0; i < max_con; i++ {
		wg.Add(1)
		time.Sleep(time.Nanosecond)
		go func() {
			defer wg.Done()

			c, e := kcp.Dial("127.0.0.1:10086")
			if nil != e {
				t.FailNow()
			}
			defer c.Close()

			c.Write(network.NewDefaultPacket([]byte("ping")).Serialize())
			b := make([]byte, 1024)
			c.SetReadDeadline(time.Now().Add(latency))
			if _, e := c.Read(b); nil != e {
				t.Fatalf("error:%s", e.Error())
			}

			// time.Sleep(time.Second)
		}()
	}

	wg.Wait()
	time.Sleep(time.Second * 2)

	n := atomic.LoadUint32(&callback.numConn)
	if n != max_con {
		t.Errorf("numConn[%d] should be [%d]", n, max_con)
	}

	n = atomic.LoadUint32(&callback.numMsg)
	if n != max_con {
		t.Errorf("numMsg[%d] should be [%d]", n, max_con)
	}

	n = atomic.LoadUint32(&callback.numDiscon)
	if n != max_con {
		t.Errorf("numDiscon[%d] should be [%d]", n, max_con)
	}
}

func Benchmark_KCPServer(b *testing.B) {

	l, err := kcp.Listen(":10086")
	if nil != err {
		panic(err)
	}

	config := &network.Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
	}

	callback := &testCallback{}
	server := network.NewServer(config, &testCallback{}, &network.DefaultProtocol{})

	go server.Start(l, func(conn net.Conn, i *network.Server) *network.Conn {
		kcpConn := conn.(*kcp.UDPSession)
		kcpConn.SetNoDelay(1, 10, 2, 1)
		kcpConn.SetStreamMode(true)
		kcpConn.SetWindowSize(4096, 4096)
		kcpConn.SetReadBuffer(4 * 1024 * 1024)
		kcpConn.SetWriteBuffer(4 * 1024 * 1024)
		kcpConn.SetACKNoDelay(true)

		return network.NewConn(conn, server)
	})

	time.Sleep(time.Millisecond * 100)

	wg := sync.WaitGroup{}
	var max_con uint32 = 0
	c, e := kcp.Dial("127.0.0.1:10086")
	if nil != e {
		b.FailNow()
	}

	go func() {
		for {
			buf := make([]byte, 1024)
			c.SetReadDeadline(time.Now().Add(time.Second * 2))
			_, er := c.Read(buf)
			if nil != er {
				// b.FailNow()
				return
			}
			wg.Done()
		}

	}()

	for i := 0; i < b.N; i++ {
		max_con++

		wg.Add(1)
		go func() {

			c.Write(network.NewDefaultPacket([]byte("ping")).Serialize())

			// time.Sleep(time.Second)
		}()
	}

	wg.Wait()
	// time.Sleep(time.Second * 2)
	server.Stop()

	n := atomic.LoadUint32(&callback.numMsg)
	b.Logf("numMsg[%d]", n)
	if n != callback.numMsg {
		b.Errorf("numMsg[%d] should be [%d]", n, max_con)
	}
	/*
		n = atomic.LoadUint32(&numConn)
		b.Logf("numConn[%d]", n)
		if n != max_con {
			b.Errorf("numConn[%d] should be [%d]", n, max_con)
		}



		n = atomic.LoadUint32(&numDiscon)
		b.Logf("numDiscon[%d]", n)
		if n != numDiscon {
			b.Errorf("numDiscon[%d] should be [%d]", n, max_con)
		}
	*/
}

func Test_TCPServer(t *testing.T) {

	l, err := net.Listen("tcp", ":10086")
	if nil != err {
		panic(err)
	}

	config := &network.Config{
		PacketReceiveChanLimit: 1024,
		PacketSendChanLimit:    1024,
		ConnReadTimeout:        time.Millisecond * 50,
		ConnWriteTimeout:       time.Millisecond * 50,
	}

	callback := &testCallback{}
	server := network.NewServer(config, callback, &network.DefaultProtocol{})

	go server.Start(l, func(conn net.Conn, i *network.Server) *network.Conn {
		return network.NewConn(conn, server)
	})

	time.Sleep(time.Second)

	wg := sync.WaitGroup{}
	const max_con = 100
	for i := 0; i < max_con; i++ {
		wg.Add(1)
		time.Sleep(time.Nanosecond)
		go func() {
			defer wg.Done()
			c, e := net.Dial("tcp", "127.0.0.1:10086")
			if nil != e {
				t.FailNow()
			}
			defer c.Close()
			c.Write(network.NewDefaultPacket([]byte("ping")).Serialize())
			b := make([]byte, 1024)
			c.SetReadDeadline(time.Now().Add(time.Second * 2))
			c.Read(b)
			// time.Sleep(time.Second)
		}()
	}

	wg.Wait()
	// time.Sleep(max_sleep)
	server.Stop()

	n := atomic.LoadUint32(&callback.numConn)
	if n != max_con {
		t.Errorf("numConn[%d] should be [%d]", n, max_con)
	}

	n = atomic.LoadUint32(&callback.numMsg)
	if n != max_con {
		t.Errorf("numMsg[%d] should be [%d]", n, max_con)
	}

	n = atomic.LoadUint32(&callback.numDiscon)
	if n != max_con {
		t.Errorf("numDiscon[%d] should be [%d]", n, max_con)
	}
}
