package net

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
	"github.com/Masterminds/glide/path"
)

type TCPServerConfig struct {
	Addr             string
	NewAgent         func(connection *TCPConnection) Agent
	MsgCodec         MessageCodec
	PenddingMsgCount int
	MaxConnCount     int
}

// TCPServer is a implement of the server conception, the server can accpet tcp connect
// new Agent for the connect, manage the client connnects and the agent servcie lifetime
type TCPServer struct {
	config     TCPServerConfig
	listenerWG sync.WaitGroup
	listener   *net.TCPListener
	connsLock  sync.Mutex
	connsWG    sync.WaitGroup
	connsMap   map[*TCPConnection]struct{}
}

// NewServer  new a server instance
func NewServer(config *TCPServerConfig) (*TCPServer, error) {
	s := new(TCPServer)
	s.config = *config

	tcpAddr, err := net.ResolveTCPAddr("tcp", s.config.Addr)
	if err != nil {
		return nil, err
	}

	if s.config.NewAgent == nil {
		return nil, errors.New("NewAgent can not be null")
	}

	if s.config.MaxConnCount == 0 {
		s.config.MaxConnCount = 1024
		log.Println("invalid MaxConnCount, set to default ", s.config.MaxConnCount)
	}

	if s.config.PenddingMsgCount == 0 {
		s.config.PenddingMsgCount = 128
		log.Println("invalid PenddingMsgCount, set to default ", s.config.PenddingMsgCount)
	}

	if s.config.MsgCodec == nil {
		s.config.MsgCodec, _ = NewDefaultMessageReaderWriter(2, 4094)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}

	s.listener = listener
	s.connsMap = make(map[*TCPConnection]struct{})

	return s, nil
}

// Serve : block run the server
func (s *TCPServer) Serve() {
	s.listenerWG.Add(1)
	defer s.listenerWG.Done()

	var retryInterval = time.Second
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("accept error : %v, retry in %v sec\n", err, retryInterval)
				time.Sleep(retryInterval)
				continue
			} else {
				return
			}
		}

		s.connsLock.Lock()
		if len(s.connsMap) >= s.config.MaxConnCount {
			s.connsLock.Unlock()
			conn.Close()
			log.Println("max connection !!!")
		}
		tcpConn := newTCPConnection(conn, s.config.PenddingMsgCount, s.config.MsgCodec)
		s.connsMap[tcpConn] = struct{}{}
		s.connsLock.Unlock()

		s.connsWG.Add(1)
		agent := s.config.NewAgent(tcpConn)
		go func() {
			defer s.connsWG.Done()

			agent.Init()
			agent.Run()
			agent.Destroy()

			tcpConn.Close()

			s.connsLock.Lock()
			delete(s.connsMap, tcpConn)
			s.connsLock.Unlock()
		}()
	}
}

// Close : close server
func (s *TCPServer) Close() {
	s.listener.Close()
	s.listenerWG.Wait()

	s.connsLock.Lock()
	for conn := range s.connsMap {
		conn.Close()
	}
	s.connsMap = nil
	s.connsLock.Unlock()

	s.connsWG.Wait()
}
