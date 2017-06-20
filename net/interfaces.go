package net

import (
	"net"
)

// MessageCodec : encode and decode message
type MessageCodec interface {
	Encodec(net.Conn) ([]byte, error)
	Decodec(net.Conn, []byte) error
}

// Agent is a delegate of the tcp connect which will run in a go rutine
type Agent interface {
	Init() bool
	Run()
	Destroy()
}
