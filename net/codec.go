package net

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math"
	"bufio"
)

var _ CodecSteam= &headBodyCoder{}
var _ CodecSteam = &textLineCoder{}

// packet format [header][body] header = len(body)
type headBodyCoder struct {
	reader *bufio.Reader
	writer io.Writer
	headerByteCount int
	headerLenParser func([]byte) uint32
	headerLenSaver  func(uint32, []byte)
	maxBodyInByte   uint32
}

func NewHeadBodyCodec(readwriter io.ReadWriter, headSize int, maxBodyInByte uint32) (*headBodyCoder, error) {
	var lenFunc func([]byte) uint32
	var saveLenFunc func(uint32, []byte)
	var maxSize uint32
	switch headSize {
	case 1:
		maxSize = math.MaxUint8
		lenFunc = func(buf []byte) uint32 { return uint32(buf[0]) }
		saveLenFunc = func(len uint32, buf []byte) { buf[0] = byte(len) }
	case 2:
		maxSize = math.MaxUint16
		lenFunc = func(buf []byte) uint32 { return uint32(binary.BigEndian.Uint16(buf)) }
		saveLenFunc = func(len uint32, buf []byte) { binary.BigEndian.PutUint16(buf, uint16(len)) }
	case 4:
		maxSize = math.MaxUint32
		lenFunc = func(buf []byte) uint32 { return binary.BigEndian.Uint32(buf) }
		saveLenFunc = func(len uint32, buf []byte) { binary.BigEndian.PutUint32(buf, len) }
	default:
		return nil, errors.New("invalid headerByteCount")
	}

	if maxBodyInByte > maxSize {
		return nil, errors.New("invalid maxMsgSize")
	}

	codec := &headBodyCoder{
		reader:bufio.NewReader(readwriter),
		writer:readwriter,
		headerByteCount:headSize,
		headerLenParser:lenFunc,
		headerLenSaver:saveLenFunc,
		maxBodyInByte:maxBodyInByte,
	}
	return codec, nil
}

func (c *headBodyCoder) Read() ([]byte, error) {

	var headerBuf = make([]byte, c.headerByteCount)
	_, err := io.ReadFull(c.reader,headerBuf)
	if err != nil {
		log.Printf("error: read header failed")
		return nil, err
	}

	msgLen := c.headerLenParser(headerBuf)
	if msgLen > c.maxBodyInByte {
		log.Printf("error: parse header failed")
		return nil, errors.New("coder: message too long")
	}

	dataBuf := make([]byte, msgLen)
	_, err = io.ReadFull(c.reader, dataBuf)
	if err != nil {
		log.Printf("error: read msg body failed")
		return nil, err
	}

	return dataBuf, nil
}

func (c *headBodyCoder) Write(msg []byte) error {
	msgLen := uint32(len(msg))
	if msgLen > c.maxBodyInByte {
		return errors.New("invalid message: message too long")
	}

	var messageBuf = make([]byte, uint32(c.headerByteCount)+msgLen)
	c.headerLenSaver(msgLen, messageBuf[:c.headerByteCount])

	copy(messageBuf[c.headerByteCount:], msg)

	_, err := c.writer.Write(messageBuf)
	return err
}

type textLineCoder struct{
	r *bufio.Reader
	w io.Writer
}

func NewTextLineCodec(readwriter io.ReadWriter) CodecSteam {
	return &textLineCoder{
		r: bufio.NewReader(readwriter),
		w: readwriter,
	}
}

func (c *textLineCoder) Read()([]byte, error) {
	var result []byte
	for {
		line, isPrefix, err := c.r.ReadLine()
		if err != nil {
			return nil, err
		} else {
			result = append(result, line...)
			if !isPrefix {
				return result, nil
			}
		}
	}
}

func (c *textLineCoder) Write(msg []byte) error {
	_, err := c.w.Write(msg)
	return err
}