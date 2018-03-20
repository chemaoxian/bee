package net

import (
	"errors"
	"log"
	"net"
	"sync"
	"context"
	"time"
)

var (
	ErrorConnectClosed    = errors.New("tcpconn: connect is close")
	ErrorSendBlocked      = errors.New("tcpconn: send queue full")
	ErrorCloseByPeer      = errors.New("tcpconn: close by peer")
	ErrorKeepaliveTimeout = errors.New(("tcpconn: keepalive timeout"))
)

type tcpConnectionOption struct {
	iosteam         CodecSteam
	agentFactory    AgentFactory
	closeCallback   func(conn *TCPConnection)
	maxWritePending int
	keepaliveTimeout time.Duration
}

// TCPConnection wrap the under net.TCPConn. Read and write package from under tcp socket
type TCPConnection struct {
	opt        *tcpConnectionOption
	ctx        context.Context
	cancelFn   context.CancelFunc
	once       sync.Once
	wg         sync.WaitGroup
	lock       sync.Mutex
	conn       net.Conn
	writeChan  chan []byte
	closeError error
}

func (c *TCPConnection) NetConn() net.Conn {
	return c.conn
}

// SendMessage : send message to connection
func (c *TCPConnection) WritePacket(message []byte) error {
	if len(message) <= 0{
		return nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.IsClosed() {
		select {
		case c.writeChan <- message:
			return nil
		default:
			log.Printf("connection %s send qeueue is full, close it", c.conn.RemoteAddr())
			return ErrorSendBlocked
		}
	} else {
		return ErrorConnectClosed
	}
}

// Close : close tcp connection
func (c *TCPConnection) Close() {
	c.CloseWithError(nil)
}

func (c *TCPConnection) CloseWithError(err error) {
	c.once.Do(func() {
		c.closeError = err
		c.opt.closeCallback(c)
		c.cancelFn()
		c.conn.Close()
		c.wg.Wait()
		close(c.writeChan)
		log.Printf("exit connection : %v", c.conn.RemoteAddr())
	})
}

func (c *TCPConnection) IsClosed() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

func newTCPConnection(parent context.Context,
	conn net.Conn,
	opt *tcpConnectionOption) *TCPConnection {

	ctx, cancel := context.WithCancel(parent)

	t := &TCPConnection{
		opt:			opt,
		ctx:           ctx,
		cancelFn:      cancel,
		conn:          conn,
		writeChan:     make(chan []byte, opt.maxWritePending),
	}

	return t
}

func (c *TCPConnection) start() {
	c.wg.Add(2)
	loopers := [2]func(){c.writeLooper, c.readLooper}
	for _, looper := range (loopers) {
		go looper()
	}
}

func (c *TCPConnection) writeLooper() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			c.Close()
			return
		case msg, ok := <-c.writeChan:
			if ok {
				err := c.opt.iosteam.Write(msg)
				if err != nil && !c.IsClosed(){
					log.Printf("send data occur error: %v, connection: %v", err, c.conn.RemoteAddr())
					c.CloseWithError(ErrorCloseByPeer)
					return
				}
			}
		}
	}
}

func (c *TCPConnection) readLooper() {
	agent := c.opt.agentFactory(c)
	defer c.wg.Done()
	coder := c.opt.iosteam
	for {
		if (c.opt.keepaliveTimeout != 0) {
			c.conn.SetReadDeadline(time.Now().Add(c.opt.keepaliveTimeout))
		}

		message, err := coder.Read()
		if err != nil && !c.IsClosed() {
			log.Println("unpack message failed, error:", err)

			if netErr, ok:= err.(net.Error); ok && netErr.Timeout() {
				c.CloseWithError(ErrorKeepaliveTimeout)
			} else {
				c.CloseWithError(ErrorCloseByPeer)
			}
			break
		}

		select {
		case <-c.ctx.Done():
			break
		default:
			agent.OnMessage(message)
		}
	}

	agent.OnDestroy()
	c.Close()
}
