package network

import (
	"net"
	"sync"
	"time"
)

type Config struct {
	PacketSendChanLimit    uint32        // the limit of packet send channel
	PacketReceiveChanLimit uint32        // the limit of packet receive channel
	ConnReadTimeout        time.Duration // read timeout
	ConnWriteTimeout       time.Duration // write timeout
}

type Server struct {
	config    *Config         // server configuration
	callback  ConnCallback    // message callbacks in connection
	protocol  Protocol        // customize packet protocol
	exitChan  chan struct{}   // notify all goroutines to shutdown
	waitGroup *sync.WaitGroup // wait for all goroutines
	closeOnce sync.Once
	listener  net.Listener
}

// NewServer creates a server
func NewServer(config *Config, callback ConnCallback, protocol Protocol) *Server {
	return &Server{
		config:    config,
		callback:  callback,
		protocol:  protocol,
		exitChan:  make(chan struct{}),
		waitGroup: &sync.WaitGroup{},
	}
}

type ConnectionCreator func(net.Conn, *Server) *Conn

// Start starts service
func (s *Server) Start(listener net.Listener, create ConnectionCreator) {
	s.listener = listener
	s.waitGroup.Add(1)
	defer func() {
		s.waitGroup.Done()
	}()

	for {
		select {
		case <-s.exitChan:
			return

		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		s.waitGroup.Add(1)
		go func() {
			create(conn, s).Do()
			s.waitGroup.Done()
		}()
	}
}

// Stop stops service
func (s *Server) Stop() {
	s.closeOnce.Do(func() {
		close(s.exitChan)
		s.listener.Close()
	})

	s.waitGroup.Wait()
}
