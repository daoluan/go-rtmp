package main

import (
	"bytes"
	"encoding/binary"
)

type FLVHeader struct {
	flv   [3]byte
	ver   byte
	sinfo byte // stream info
	len   uint32
}

func (f *FLVHeader) toBytes() []byte {
	var b [9]byte
	b[0] = f.flv[0]
	b[1] = f.flv[1]
	b[2] = f.flv[2]
	b[3] = f.ver
	b[4] = f.sinfo
	binary.BigEndian.PutUint32(b[5:], f.len)

	return b[:]
}

type FLVTagHeader struct {
	t        uint8
	size     uint32 // 3B
	ts       uint32 // 3B
	exts     uint8
	streamid uint32 // 3B
}

func (f *FLVTagHeader) toBytes() []byte {
	var b [11]byte
	b[0] = f.t

	b[1] = uint8(f.size >> 16)
	b[2] = uint8(f.size >> 8)
	b[3] = uint8(f.size)

	b[4] = uint8(f.ts >> 16)
	b[5] = uint8(f.ts >> 8)
	b[6] = uint8(f.ts)

	b[7] = 0

	b[8] = uint8(f.streamid >> 16)
	b[9] = uint8(f.streamid >> 8)
	b[10] = uint8(f.streamid)
	return b[:]
}

func PackFlvTag(ft *bytes.Buffer, msgtype uint8, timestamp uint32,
	body bytes.Buffer) {
	var fh FLVTagHeader
	fh.t = msgtype
	fh.size = uint32(body.Len())
	fh.streamid = 0
	fh.ts = timestamp

	binary.Write(ft, binary.LittleEndian, fh.toBytes())
	binary.Write(ft, binary.LittleEndian, body.Bytes())
}
