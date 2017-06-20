package net

import (
	"sync"
	"net"
	"errors"
	"log"
	"time"
)

type TCPClientConfig struct {
	Addr string
	NewAgent func(connection *TCPConnection) Agent
	MsgCodec MessageCodec
	PenddingMsgCount int
	AutoConnect bool
	ConnectInterval time.Duration
}

type TCPClient struct {
	conf TCPClientConfig
	closeWG sync.WaitGroup
	closeChan chan bool
	rawConn *net.TCPConn
}

func NewTCPClient(conf *TCPClientConfig) (*TCPClient, error){
	c := new(TCPClient)
	c.conf = *conf
	c.closeChan = make(chan bool)

	if c.conf.NewAgent == nil {
		return nil, errors.New("NewAgent can not be null")
	}

	if c.conf.PenddingMsgCount == 0 {
		c.conf.PenddingMsgCount = 128
		log.Println("invalid PenddingMsgCount, set to default ", c.conf.PenddingMsgCount)
	}

	if c.conf.MsgCodec == nil {
		c.conf.MsgCodec, _ = NewDefaultMessageReaderWriter(2, 4094)
	}

	return c, nil
}

func (c *TCPClient) connect() error {
	conn, err := net.Dial("tcp", c.conf.Addr)
	if err != nil {
		return err
	}
	c.rawConn = conn.(*net.TCPConn)
	return nil
}

func (c *TCPClient) Run() {
	c.closeWG.Add(1)
	defer c.closeWG.Done()

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

		tcpConn := newTCPConnection(c.rawConn, c.conf.PenddingMsgCount, c.conf.MsgCodec)
		agent := c.conf.NewAgent(tcpConn)

		if !agent.Init() {
			tcpConn.Close()

			if !c.conf.AutoConnect && c.IsClose()  {
				break
			}
			time.Sleep(c.conf.ConnectInterval)
			continue
		}

		agent.Run()
		agent.Destroy()

		if !c.conf.AutoConnect && c.IsClose() {
			break
		}
		time.Sleep(c.conf.ConnectInterval)
	}
}

func (c* TCPClient) IsClose() bool {
	select {
		case <- c.closeChan:
			return true
	default:
		return false
	}
}

func (c* TCPClient) Close() {
	if !c.IsClose() {
		close(c.closeChan)
		c.rawConn.Close()
	}
}