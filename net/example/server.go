package main

import (
	"bee"
	"fmt"
	"log"
)

func main() {
	cfg := bee.TCPServerConfig{
		Addr:         ":22222",
		AgentFactory: newAgent,
		SteamFactory: bee.NewTextLineCodec,
		MaxConnCount: 1,
	}

	s, err := bee.NewServer(&cfg)
	if err != nil {
		fmt.Println("start server failed, err:", err.Error())
		return
	}

	s.Serve()
}

func newAgent(conn bee.Connection) bee.Agent {
	log.Println("new agent, addr : ", conn.NetConn().RemoteAddr().String())
	return &Agent{
		conn,
	}
}

type Agent struct {
	conn bee.Connection
}

func (a *Agent) OnMessage(message []byte) {
	log.Println("OnMessage : ", string(message))
	a.conn.WritePacket(append(message,"\n"...))
}

func (a *Agent) OnDestroy() {
	log.Println("OnDestroy")
}
