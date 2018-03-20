package net

import (
	"log"
	"net"
	"sync"
	"time"
	"sync/atomic"
	"context"
	"bee/net/netutil"
	"io"
)

type TCPServerConfig struct {
	Addr             string
	SteamFactory     CodecSteamFactory
	AgentFactory     AgentFactory
	PenddingMsgCount int
	MaxConnCount     int
	KeepaliveTimeout time.Duration
}

// TCPServer is a implement of the server conception, the server can accpet tcp connect
// new Agent for the connect, manage the client connnects and the agent servcie lifetime
type TCPServer struct {
	config     TCPServerConfig
	listener   net.Listener
	connsLock  sync.Mutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFn   context.CancelFunc
	connsMap   sync.Map
	totalCount int32
}

// NewServer  new a server instance
func NewServer(config *TCPServerConfig) (*TCPServer, error) {
	s := new(TCPServer)
	s.config = *config

	tcpAddr, err := net.ResolveTCPAddr("tcp", s.config.Addr)
	if err != nil {
		return nil, err
	}

	if s.config.AgentFactory == nil {
		log.Println("agent factory can not be nil")
		return nil, err
	}

	if s.config.MaxConnCount == 0 {
		s.config.MaxConnCount = 1024
		log.Println("invalid MaxConnCount, set to default ", s.config.MaxConnCount)
	}

	if s.config.PenddingMsgCount == 0 {
		s.config.PenddingMsgCount = 128
		log.Println("invalid PenddingMsgCount, set to default ", s.config.PenddingMsgCount)
	}

	if s.config.SteamFactory == nil {
		s.config.SteamFactory = func (rw io.ReadWriter) CodecSteam {
			coder, _:= NewHeadBodyCodec(rw,2, 4094)
			return  coder
		}
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}

	s.listener = netutil.LimitListener(listener, s.config.MaxConnCount)
	s.ctx, s.cancelFn = context.WithCancel(context.Background())
	return s, nil
}

// Serve : block run the server
func (s *TCPServer) Serve() error {
	s.wg.Wait()
	defer s.wg.Done()

	var retryInterval = time.Second
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("accept error : %v, retry in %v sec\n", err, retryInterval)
				time.Sleep(retryInterval)
				continue
			} else {
				return err
			}
		}

		if int(s.GetConectionCount()) >= s.config.MaxConnCount {
			conn.Close()
			log.Println("accept error : max connect exceed")
			continue
		}

		s.startSession(conn)
	}
}

func (s *TCPServer) IsClosed() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
}

// Close : close server
func (s *TCPServer) Close() {
	if s.IsClosed() {
		return
	}

	s.connsLock.Lock()
	s.cancelFn()
	s.listener.Close()
	s.connsLock.Unlock()

	s.wg.Wait()
}

func (s *TCPServer) GetConectionCount() int32 {
	return atomic.LoadInt32(&(s.totalCount))
}

func (s *TCPServer) startSession(conn net.Conn) {
	s.wg.Add(1)
	opt := &tcpConnectionOption{
		iosteam:          s.config.SteamFactory(conn),
		agentFactory:     s.config.AgentFactory,
		closeCallback:    s.removeSession,
		maxWritePending : s.config.PenddingMsgCount,
		keepaliveTimeout: s.config.KeepaliveTimeout,
	}
	tcpConn := newTCPConnection(s.ctx, conn, opt)
	tcpConn.start()

	atomic.AddInt32(&(s.totalCount), 1)
	s.connsMap.Store(tcpConn, struct{}{})
}

func (s *TCPServer) removeSession(conn *TCPConnection) {
	s.wg.Done()
	s.connsMap.Delete(conn)
	atomic.AddInt32(&(s.totalCount), -1)
}
