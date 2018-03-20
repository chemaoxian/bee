package net

import (
	"sync"
	"net"
	"errors"
	"log"
	"time"
	"context"
	"io"
)

type TCPClientConfig struct {
	Addr              string
	AgentFactory      AgentFactory
	CodecSteamFactory CodecSteamFactory
	PenddingMsgCount  int
	AutoConnect       bool
	ConnectInterval   time.Duration
}

type TCPClient struct {
	conf     TCPClientConfig
	ctx      context.Context
	cancelFn context.CancelFunc
	connLock sync.Mutex
	conn  *TCPConnection
	closeMsgChan chan struct{}
	closeOnce sync.Once
}

func NewTCPClient(conf *TCPClientConfig) (*TCPClient, error){
	c := new(TCPClient)
	c.conf = *conf

	if c.conf.AgentFactory == nil {
		log.Println("New tcp client failed, AgentFactory = nil ", c.conf.PenddingMsgCount)
		return nil, errors.New("AgentFactory can not be null")
	}

	if c.conf.PenddingMsgCount == 0 {
		c.conf.PenddingMsgCount = 128
		log.Println("invalid PenddingMsgCount, set to default ", c.conf.PenddingMsgCount)
	}

	if c.conf.CodecSteamFactory == nil {
		c.conf.CodecSteamFactory = func(rw io.ReadWriter) CodecSteam{
			coder, _ := NewHeadBodyCodec(rw,2, 4094)
			return coder
		}
	}

	c.ctx, c.cancelFn = context.WithCancel(context.Background())
	c.closeMsgChan = make(chan struct{})
	return c, nil
}

func (c *TCPClient) connect() error {
	conn, err := net.Dial("tcp", c.conf.Addr)
	if err != nil {
		return err
	}

	opt := &tcpConnectionOption{
		c.conf.CodecSteamFactory(conn),
		c.conf.AgentFactory,
		c.connectClosed,
		c.conf.PenddingMsgCount,
		0,
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	c.conn = newTCPConnection(c.ctx, conn, opt)
	c.conn.start()

	return nil
}

func (c *TCPClient) Run() {
	for {
		if c.IsClose() {
			break
		}
		err := c.connect()
		log.Printf("connect to %v, err : %v\n", c.conf.Addr, err)
		if err != nil {
			if !c.conf.AutoConnect || c.IsClose() {
				break
			}
			time.Sleep(c.conf.ConnectInterval)
			continue
		}

		select {
			case <- c.ctx.Done():
				break
			case <-	c.closeMsgChan:
				if !c.conf.AutoConnect {
					break
				}
				time.Sleep(c.conf.ConnectInterval)
		}
	}
}

func (c *TCPClient) connectClosed(connection * TCPConnection) {
	c.closeMsgChan<- struct{}{}
}

func (c* TCPClient) IsClose() bool {
	select {
		case <- c.ctx.Done():
			return true
	default:
		return false
	}
}

func (c *TCPClient) GetConnection() *TCPConnection {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	return c.conn
}

func (c* TCPClient) Close() {
	c.closeOnce.Do(func(){
		c.cancelFn()

		var conn *TCPConnection
		c.connLock.Lock()
		if c.conn != nil {
			conn = c.conn
			c.conn = nil
		}
		c.connLock.Unlock()
		conn.Close()
		close(c.closeMsgChan)
		c.closeMsgChan = nil
	})
}