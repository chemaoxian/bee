package net

import (
	"errors"
	"log"
	"net"
	"sync"
	"github.com/Masterminds/glide/path"
	"syscall"
	"io"
	"go/format"
)

var (
	ErrorConnectClosed = errors.New("connect is close")
	ErrorNullMessage = errors.New("null message")
	ErrorSendBlocked = errors.New("send queue full")
)
// TCPConnection implement the conception of connect, read msg, write msg
type TCPConnection struct {
	waitGroup sync.WaitGroup
	lock      sync.Mutex
	conn      *net.TCPConn
	closeChan chan bool
	writeChan chan []byte
	codec     MessageCodec
}

func newTCPConnection(conn *net.TCPConn, pendingMsgCount int, msgCodec MessageCodec) *TCPConnection {
	c := &TCPConnection{
		conn:      conn,
		closeChan: make(chan bool, 1),
		writeChan: make(chan []byte, pendingMsgCount),
		codec:     msgCodec,
	}

	c.waitGroup.Add(1)
	go func() {
		defer c.waitGroup.Done()
		for {
			select {
			case msg, ok := <-c.writeChan:
				if ok {
					err := c.codec.Decodec(c.conn, msg)
					if err != nil {
						log.Printf("send data occur error: %v, connection: %v", err, c.conn.RemoteAddr())
						c.Close()
						return
					}
				}
				case <-c.closeChan:
					log.Printf("exit send go rutine, connection closed : %v", c.conn.RemoteAddr())
					return
			}
		}
	}()

	return c
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

	if !c.IsClosed(){
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

func (c *TCPConnection) GetMessage() ([]byte, error) {
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
