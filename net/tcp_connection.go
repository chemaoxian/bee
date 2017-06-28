package net

import (
	"errors"
	"log"
	"net"
	"sync"
	"github.com/lukehoban/ident"
)

var (
	ErrorConnectClosed = errors.New("connect is close")
	ErrorNullMessage   = errors.New("null message")
	ErrorSendBlocked   = errors.New("send queue full")
)

// TCPConnection implement the conception of connect, read msg, write msg
type TCPConnection struct {
	waitGroup sync.WaitGroup
	lock      sync.Mutex
	conn      *net.TCPConn
	closeChan chan bool
	writeChan chan []byte
	sendChan chan []byte
	codec     MessageCodec
	id 		uint32
}

func newTCPConnection( id uint32,
						conn *net.TCPConn,
						pendingMsgCount int,
						msgCodec MessageCodec) *TCPConnection {
	return &TCPConnection{
		conn:      conn,
		closeChan: make(chan bool, 1),
		writeChan: make(chan []byte, pendingMsgCount),
		sendChan: make(chan []byte),
		codec:     msgCodec,
		id:	id,
	}
}

func (me *TCPConnection) start() {
	loopers := [2]func(){me.writeLooper, me.readLooper}
	for _, looper := range(loopers) {
		me.waitGroup.Add(1)
		go looper()
	}
}

func (me *TCPConnection) writeLooper() {
	defer me.waitGroup.Done()
	for {
		select {
		case msg, ok := <-me.writeChan:
			if ok {
				err := me.codec.Decodec(me.conn, msg)
				if err != nil {
					log.Printf("send data occur error: %v, connection: %v", err, me.conn.RemoteAddr())
					me.Close()
					return
				}
			}
		case <-me.closeChan:
			log.Printf("exit send go rutine, connection closed : %v", me.conn.RemoteAddr())
			return
		}
	}
}

func (me *TCPConnection) readLooper(){
	defer me.waitGroup.Done()
	for {
		msgBuffer, err := me.getMessage()
		if err != nil {
			log.Printf("get message failed, error : %v\n", err)
			me.Close()
			break
		}

		select {
			case <-me.closeChan:
				log.Printf("exit read looper, connection : %s", me.RemoteAddr())
				return
			case me.sendChan<- msgBuffer:
		}
	}
}
// LocalAddr : get local addr
func (c *TCPConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr : get remote addr
func (c *TCPConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SendMessage : send message to connection
func (c *TCPConnection) SendMessage(message []byte) error {
	if message == nil {
		return ErrorNullMessage
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.IsClosed() {
		select {
		case c.writeChan <- message:
			return nil
		default:
			log.Printf("connection %s send qeueue is full, close it", c.RemoteAddr())
			c.Close()
			return ErrorSendBlocked
		}
	} else {
		return ErrorConnectClosed
	}
}

func (c *TCPConnection) getMessage() ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.IsClosed() {
		return nil, ErrorConnectClosed
	}

	message, err := c.codec.Encodec(c.conn)
	if err != nil {
		log.Printf("get message failed: %v, connection: %v", err, c.conn.RemoteAddr())
		c.Close()
		return nil, err
	}

	return message, err
}

func (me *TCPConnection) GetMesageChan() <- chan []byte {
	return me.sendChan
}

func (me *TCPConnection) ConnectionId() uint32 {
	return me.id
}

// Close : close tcp connection
func (c *TCPConnection) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.IsClosed() {
		close(c.closeChan)

		go func() {
			log.Printf("exit connection : %v", c.conn.RemoteAddr())
			c.waitGroup.Wait()
			close(c.writeChan)
			c.conn.Close()
		}()
	}
}

func (c *TCPConnection) IsClosed() bool {
	select {
	case <-c.closeChan:
		return true
	default:
		return false
	}
}
