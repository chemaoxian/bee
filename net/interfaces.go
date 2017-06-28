package net

import (
	"net"
)

// MessageCodec : encode and decode message
type MessageCodec interface {
	Encodec(net.Conn) ([]byte, error)
	Decodec(net.Conn, []byte) error
}

type Message struct {
	Content []byte
	ConnectionId uint32
	ServerId	uint32
}

// Agent is a delegate of the tcp connect which will run in a go rutine
type Agent interface {
	Init()
	Run()
	Destroy()
}