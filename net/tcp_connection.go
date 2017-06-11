package net

import (
	"errors"
	"log"
	"net"
	"sync"
)

// TCPConnection implete the conception of connect, read msg, write msg
type TCPConnection struct {
	lock            sync.Mutex
	conn            *net.TCPConn
	closeFlag       bool
	writeChan       chan []byte
	msgReaderWriter MessageReaderWriter
}

func newTCPConnection(conn *net.TCPConn, pendingMsgCount int, msgCodec MessageReaderWriter) *TCPConnection {
	c := &TCPConnection{
		conn:            conn,
		closeFlag:       false,
		writeChan:       make(chan []byte, pendingMsgCount),
		msgReaderWriter: msgCodec,
	}

	go func() {
		for msg := range c.writeChan {
			if msg != nil {
				err := c.msgReaderWriter.Write(c.conn, msg)
				if err != nil {
					log.Printf("send data occur error: %v, connection: %v", err, c.conn.RemoteAddr())
					break
				}
			} else {
				break
			}
		}

		c.lock.Lock()
		c.conn.Close()
		c.closeFlag = true
		c.lock.Unlock()
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
func (c *TCPConnection) SendMessage(message []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.closeFlag && message != nil {
		c.doSendMessge(message)
	}
}

// GetMessage : get message from connnection
func (c *TCPConnection) GetMessage() ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.closeFlag {
		return c.msgReaderWriter.Read(c.conn)
	}

	return nil, errors.New("connect already close")
}

// Close : close tcp connection
func (c *TCPConnection) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.closeFlag {
		c.doSendMessge(nil)
		c.closeFlag = true
	}
}

// Destroy : destroy tcp connection
func (c *TCPConnection) Destroy() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.doDestory()
}

func (c *TCPConnection) doSendMessge(message []byte) {
	if len(c.writeChan) == cap(c.writeChan) {
		log.Printf("close conn: send queue full")
		c.doDestory()
		return
	}

	c.writeChan <- message
}

func (c *TCPConnection) doDestory() {
	if !c.closeFlag {
		c.conn.Close()
		close(c.writeChan)
		c.closeFlag = true
	}
}
