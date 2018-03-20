package net_test

import (
	"testing"
	"bytes"
	"bee/net"
	"encoding/binary"
)

func TestHeadBodyRead(t *testing.T) {
	var content = []byte("0123456789abcdef")
	head := make([]byte, 2)
	binary.BigEndian.PutUint16(head, uint16(len(content)))
	data := append(head, content...)

	c, _ := net.NewHeadBodyCodec(bytes.NewBuffer(data), 2, 1024)
	b, err:= c.Read()
	if (err != nil) {
		t.Errorf("TEST read head body failed, create codec failed, err=>%s", err.Error())
	}

	if bytes.Compare(b, content) != 0 {
		t.Errorf("head body codec test failed, content not the same, err=>%s", err.Error())
	}

	b1, err1 := c.Read()
	if err1 == nil {
		t.Errorf("head body codec test failed, read more data, data =>%s", string(b1))
	}
}

func TestHeadBodyCoderWrite(t *testing.T) {
	 var content = []byte("0123456789abcdef")
	head := make([]byte, 2)
	binary.BigEndian.PutUint16(head, uint16(len(content)))
	data := append(head, content...)

	buf := bytes.NewBuffer([]byte{})
	c, _ := net.NewHeadBodyCodec(buf, 2, 1024)
	err := c.Write(content)
	if (err != nil) {
		t.Errorf("TEST read head body failed, create codec failed, err=>%s", err.Error())
	}

	if bytes.Compare(data, buf.Bytes()) != 0 {
		t.Errorf("head body codec test failed, content not the same, err=>%s", err.Error())
	}
}
