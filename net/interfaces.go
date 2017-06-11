package net

import (
	"net"
)

// MessageReaderWriter : encode and decode message
type MessageReaderWriter interface {
	Read(net.Conn) ([]byte, error)
	Write(net.Conn, []byte) error
}

// Agent is a delegate of the tcp connect which will run in a go rutine
type Agent interface {
	Run()
	Destroy()
}
