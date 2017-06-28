package net

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math"
	"net"
)

var _ MessageCodec = &DefaultMessageCodec{}

// DefaultMessageCodec is a default message codec implement [header][data]
type DefaultMessageCodec struct {
	headerByteCount int
	headerLenParser func([]byte) uint32
	headerLenSaver  func(uint32, []byte)
	maxMsgSize      uint32
}

func NewDefaultMessageReaderWriter(headerByteCount int, maxMsgSize uint32) (*DefaultMessageCodec, error) {
	var lenFunc func([]byte) uint32
	var saveLenFunc func(uint32, []byte)
	var maxSize uint32
	switch headerByteCount {
	case 1:
		maxSize = math.MaxUint8
		lenFunc = func(buf []byte) uint32 { return uint32(buf[0]) }
		saveLenFunc = func(len uint32, buf []byte) { buf[0] = byte(len) }
	case 2:
		maxSize = math.MaxUint16
		lenFunc = func(buf []byte) uint32 { return uint32(binary.LittleEndian.Uint16(buf)) }
		saveLenFunc = func(len uint32, buf []byte) { binary.LittleEndian.PutUint16(buf, uint16(len)) }
	case 4:
		maxSize = math.MaxUint32
		lenFunc = func(buf []byte) uint32 { return binary.LittleEndian.Uint32(buf) }
		saveLenFunc = func(len uint32, buf []byte) { binary.LittleEndian.PutUint32(buf, len) }
	default:
		return nil, errors.New("invalid headerByteCount")
	}

	if maxMsgSize > maxSize {
		return nil, errors.New("invalid maxMsgSize")
	}

	codec := &DefaultMessageCodec{
		headerByteCount,
		lenFunc,
		saveLenFunc,
		maxMsgSize,
	}
	return codec, nil
}

func (codec *DefaultMessageCodec) Encodec(conn net.Conn) ([]byte, error) {

	var headerBuf = make([]byte, codec.headerByteCount)
	_, err := io.ReadFull(conn, headerBuf)
	if err != nil {
		log.Printf("error: read header failed")
		return nil, err
	}

	msgLen := codec.headerLenParser(headerBuf)
	if msgLen > codec.maxMsgSize {
		log.Printf("error: parse header failed")
		return nil, errors.New("invalid message: message too long")
	}

	dataBuf := make([]byte, msgLen)
	_, err = io.ReadFull(conn, dataBuf)
	if err != nil {
		log.Printf("error: read full failed")
		return nil, err
	}

	return dataBuf, nil
}

func (codec *DefaultMessageCodec) Decodec(conn net.Conn, msg []byte) error {
	msgLen := uint32(len(msg))
	if msgLen > codec.maxMsgSize {
		return errors.New("invalid message: message too long")
	}

	var messageBuf = make([]byte, uint32(codec.headerByteCount)+msgLen)
	codec.headerLenSaver(msgLen, messageBuf[:codec.headerByteCount])

	copy(messageBuf[codec.headerByteCount:], msg)

	_, err := conn.Write(messageBuf)
	return err
}
