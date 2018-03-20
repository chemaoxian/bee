package net

import (
	"net"
	"io"
)

type Connection interface {
	NetConn() net.Conn
	WritePacket([]byte) error
	Close()
	IsClosed() bool
}

type CodecSteamFactory func(io.ReadWriter) CodecSteam
type AgentFactory func(Connection) Agent

type CodecSteam interface {
	Read() ([]byte, error)
	Write([]byte) error
}

type Agent interface {
	OnMessage([]byte)
	OnDestroy()
}
