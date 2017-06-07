package main

import (
	"bytes"
	"encoding/binary"
	"go_rtmp_srv/amf"
	"log"
	"net"

	"github.com/davecgh/go-spew/spew"
)

type MessageHeader struct {
	timestamp   uint32
	msglen      uint32
	msgtype     int
	msgstreamid uint32
}

type Message struct {
	payload bytes.Buffer
}

const (
	RTMP_HS_NONE  = -1
	RTMP_HS_C0    = 0
	RTMP_HS_C1    = 1
	RTMP_HS_DONE  = 2
	RTMP_FEED_MSG = 4
)

const (
	RTMP_MSG_TYPEID_SET_PKT_SIZE     = 0x01 // Set Packet Size Message.
	RTMP_MSG_TYPEID_PING_MSG         = 0x04 // Ping Message.
	RTMP_MSG_TYPEID_SERVER_BINDWIDTH = 0x05 // Server Bandwidth
	RTMP_MSG_TYPEID_CLIENT_BINDWIDTH = 0x06 // Client Bandwidth.
	RTMP_MSG_TYPEID_AUDIO_PKT        = 0x08 // Audio Packet.
	RTMP_MSG_TYPEID_VIDEO_PKT        = 0x09 // Video Packet.
	RTMP_MSG_TYPEID_AMF3             = 0x11 // An AMF3 type command.
	RTMP_MSG_TYPEID_INVIKE           = 0x12 // Invoke (onMetaData info is sent as such).
	RTMP_MSG_TYPEID_AMF0             = 0x14 // An AMF0 type command
)

var message_type_id = map[uint32]string{
	RTMP_MSG_TYPEID_SET_PKT_SIZE: "Control message",
	2: "Control message",
	RTMP_MSG_TYPEID_AUDIO_PKT: "Audio message",
	RTMP_MSG_TYPEID_VIDEO_PKT: "Video message",
	RTMP_MSG_TYPEID_INVIKE:    "onMetaData",
	RTMP_MSG_TYPEID_AMF0:      "AMF0",
	22:                        "Aggregate message",
}

type RtmpConf struct {
	server_addr net.TCPAddr
}

type RtmpServer struct {
}

type BasicHeader struct {
	fmt  int
	csid int
}

type RtmpMessage struct {
	msgtype    int
	payloadlen int
	timestamp  uint32
	streamid   int
}

type Trunk struct {
	basic_header   BasicHeader
	message_header MessageHeader
	payload        Message
	was            int // window acknowledgement size
	bindwidth      int
}

func (t *Trunk) Init() {
	t.basic_header.fmt = -1
	t.payload.payload.Reset()
}

type RtmpConn struct {
	reqbuf             bytes.Buffer
	state              int
	preceding_streamid uint32
	preceding_msglen   uint32
	preceding_ts       uint32
	conn               net.Conn
	avc_pkt_type       bytes.Buffer
	acc_pkt_type       bytes.Buffer
	streamname         string

	trunk_size     uint32
	was            int
	bindwidth      int
	trunk          Trunk
	exit           bool
	stream_created bool
}

func HandleNewConnection(conn net.Conn) {
	var rc RtmpConn
	rc.handleNewConnection(conn)
	defer conn.Close()
}

func (r *RtmpConn) handleNewConnection(conn net.Conn) {
	r.conn = conn
	r.state = RTMP_HS_NONE
	r.trunk_size = 128
	r.exit = false
	r.stream_created = false

	if !r.handShake() {
		log.Println("fail hand shake")
		return
	}

	for !r.exit {
		if r.feed() < 0 {
			log.Println("fail to feed")
			return
		}

		r.preceding_streamid = r.trunk.message_header.msgstreamid
		r.preceding_msglen = r.trunk.message_header.msglen
		r.preceding_ts = r.trunk.message_header.timestamp
		log.Printf("msg complete(%s): %spreceding_streamid=%d|preceding_msglen=%d|preceding_ts=%d\n",
			message_type_id[uint32(r.trunk.message_header.msgtype)],
			spew.Sdump(r.trunk.message_header),
			r.preceding_streamid, r.preceding_msglen, r.preceding_ts)

		r.handleMessage()
	}

	log.Println("stream is over")

}

func (r *RtmpConn) handShake() bool {
	var recvbuf [1024]byte
	var rspbuf [1536]byte
	var feedbuf bool = true

	for {
		if feedbuf {
			len, err := r.conn.Read(recvbuf[0:])
			if err != nil {
				log.Println("fail to Read: ", err.Error())
				return false
			}
			r.reqbuf.Write(recvbuf[0:len])
			feedbuf = false
		}

		if r.state == RTMP_HS_NONE {
			log.Println("c0...")
			var ver uint8 = r.reqbuf.Next(1)[0]

			rspbuf[0] = ver

			log.Println("s0...", ver)

			r.conn.Write(rspbuf[0:1])
			r.state = RTMP_HS_C0 // c0 done
		} else if r.state == RTMP_HS_C0 {
			if r.reqbuf.Len() < 1536 {
				log.Println("the c1 is not complete")
				feedbuf = true
				continue
			}
			log.Println("c1...")

			var ts uint32 = binary.BigEndian.Uint32(r.reqbuf.Next(4)[0:4])
			r.reqbuf.Next(1532)

			log.Printf("c1 rtmp, timestamp = %d\n", ts)
			log.Println("s1...")

			copy(rspbuf[0:], r.reqbuf.Bytes())
			binary.BigEndian.PutUint32(rspbuf[0:], ts)
			r.conn.Write(rspbuf[0:1536])
			r.state = RTMP_HS_C1 // c1 done
		} else if r.state == RTMP_HS_C1 {
			if r.reqbuf.Len() < 1536 {
				log.Println("the c2 is not complete")
				feedbuf = true
				continue
			}

			log.Println("enter c2")
			var ts1 = binary.BigEndian.Uint32(r.reqbuf.Next(4)[0:4])
			var ts2 = binary.BigEndian.Uint32(r.reqbuf.Next(4)[0:4])
			r.reqbuf.Next(1528)

			copy(rspbuf[0:], r.reqbuf.Bytes())
			log.Printf("c2 rtmp, timestamp1 = %d, timestamp2 = %d\n", ts1,
				ts2)
			binary.BigEndian.PutUint32(rspbuf[0:], ts1)
			binary.BigEndian.PutUint32(rspbuf[4:], ts2)
			binary.BigEndian.PutUint32(rspbuf[8:], 345345435)

			r.conn.Write(rspbuf[0:1536])
			r.state = RTMP_HS_DONE
			break
		}
	}

	r.state = RTMP_FEED_MSG
	return true
}

// return nparsed
// -1 error close connection
// 1 not enouth
// 0 success
func (r *RtmpConn) feed() int {
	r.trunk.Init()

	var pos uint32 = 0
	var feedbuf bool = false

	for {
		log.Printf("parsed(pos)=%d|msglen=%d|len(msg)=%d|len(leftmsg)=%d\n", pos,
			r.trunk.message_header.msglen, r.trunk.payload.payload.Len(),
			r.trunk.message_header.msglen-uint32(r.trunk.payload.payload.Len()))

		if !feedbuf {
			r.reqbuf.Next(int(pos))
		}

		var recvbuf [1024]byte
		if feedbuf || r.reqbuf.Len() == 0 {
			len, err := r.conn.Read(recvbuf[0:])
			if err != nil {
				log.Println("fail to read:", err.Error())
				return -1
			}

			log.Printf("ready to parse: len(reqqbuf)=%d|readlen=%d\n", r.reqbuf.Len(), len)
			r.reqbuf.Write(recvbuf[0:len])
			feedbuf = false

		} else {
			log.Printf("ready to parse: len(reqbuf)=%d\n", r.reqbuf.Len())
		}

		pos = 0

		var reqlen uint32 = uint32(r.reqbuf.Len())
		reqbuf := r.reqbuf.Bytes()

		var rfmt uint8 = ((uint8(reqbuf[pos])) >> 6) & 0x3
		var last6b uint8 = (uint8(reqbuf[pos])) & 63
		var csid uint32 = 0

		if last6b != 0 && last6b != 1 {
			csid = uint32(last6b)
			pos += 1
		} else if last6b == 0 {
			if reqlen-pos < 2 {
				feedbuf = true
				continue
			}

			csid = uint32(reqbuf[1])
			pos += 2
		} else if last6b == 1 {
			if reqlen-pos < 3 {
				feedbuf = true
				continue
			}

			csid = uint32(binary.BigEndian.Uint16(reqbuf[pos+1 : pos+3]))
			pos += 3
		}

		if 0 == rfmt || 1 == rfmt || 2 == rfmt {
			r.trunk.basic_header.fmt = int(rfmt)
		}

		if 1 == rfmt || 2 == rfmt {
			r.trunk.basic_header.csid = int(csid)
		}

		log.Printf("csid=%d|rfmt=%d|last6b=%d\n", csid, rfmt, last6b)

		if rfmt == 0 {
			if reqlen-pos < 11 { // len(message header) == 11
				log.Println("fmt=0 message header is not complete")
				feedbuf = true
				continue
			}

			r.trunk.message_header.timestamp = uint32(reqbuf[pos]) +
				uint32(reqbuf[pos+1])*256 + uint32(reqbuf[pos+2])*256*256
			pos += 3

			r.trunk.message_header.msglen = uint32(reqbuf[pos])*256*256 +
				uint32(reqbuf[pos+1])*256 + uint32(reqbuf[pos+2])
			pos += 3

			r.trunk.message_header.msgtype = int(reqbuf[pos])
			pos += 1

			r.trunk.message_header.msgstreamid = binary.LittleEndian.Uint32(reqbuf[pos : pos+4])
			pos += 4

			log.Printf("timestamp=%d|messagelen=%d|typeid=%d|string(typeid)=%s|streamid=%d|len(reqbuf)=%d\n",
				r.trunk.message_header.timestamp,
				r.trunk.message_header.msglen,
				r.trunk.message_header.msgtype,
				message_type_id[uint32(r.trunk.message_header.msgtype)],
				csid,
				reqlen-pos)

			if r.trunk.message_header.msglen < r.trunk_size {
				// over
				if r.trunk.message_header.msglen <= uint32(reqlen-pos) {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian,
						reqbuf[pos:pos+r.trunk.message_header.msglen])
					pos += r.trunk.message_header.msglen
					break
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			} else {
				if reqlen-pos >= r.trunk_size {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk_size])
					pos += r.trunk_size
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			}

		} else if rfmt == 1 {
			if reqlen-pos < 7 {
				log.Println("fmt=1 message header is not complete: reqlen=", reqlen)
				feedbuf = true
				continue
			}

			r.trunk.message_header.msgstreamid = r.preceding_streamid

			r.trunk.message_header.timestamp = uint32(reqbuf[pos])*256*256 +
				uint32(reqbuf[pos+1])*256 + uint32(reqbuf[pos+2])
			pos += 3

			r.trunk.message_header.msglen = uint32(reqbuf[pos])*256*256 +
				uint32(reqbuf[pos+1])*256 + uint32(reqbuf[pos+2])
			pos += 3

			r.trunk.message_header.msgtype = int(reqbuf[pos])
			pos += 1

			if r.trunk.message_header.msglen < r.trunk_size {
				if r.trunk.message_header.msglen <= uint32(reqlen-pos) {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk.message_header.msglen])
					pos += r.trunk.message_header.msglen
					break
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			} else {
				if reqlen-pos >= r.trunk_size {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk_size])
					pos += r.trunk_size
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			}

		} else if rfmt == 2 {
			if reqlen-pos < 3 {
				log.Println("message header is not complete")
				feedbuf = true
				continue
			}

			r.trunk.message_header.msglen = r.preceding_msglen
			r.trunk.message_header.msgstreamid = r.preceding_streamid
			r.trunk.message_header.timestamp = uint32(reqbuf[pos])*256*256 +
				uint32(reqbuf[pos+1])*256 + uint32(reqbuf[pos+2]) + r.preceding_ts

			pos += 3

			if r.trunk.message_header.msglen < r.trunk_size {
				if r.trunk.message_header.msglen <= uint32(reqlen-pos) {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk.message_header.msglen])
					pos += r.trunk.message_header.msglen
					break
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			} else {
				if uint32(reqlen-pos) >= r.trunk_size {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk_size])
					pos += r.trunk_size
				} else {
					log.Println("recv buf is not enough continue to read")
					feedbuf = true
					continue
				}
			}

		} else if rfmt == 3 {

			if uint32(r.trunk.payload.payload.Len())+r.trunk_size < r.trunk.message_header.msglen {
				// no message header
				if reqlen-pos < r.trunk_size {
					feedbuf = true
					log.Println("fmt=3 message header is not complete")
					continue
				}

				binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+r.trunk_size])
				pos += r.trunk_size
			} else {
				left := r.trunk.message_header.msglen - uint32(r.trunk.payload.payload.Len())
				if left <= reqlen-pos {
					binary.Write(&r.trunk.payload.payload, binary.LittleEndian, reqbuf[pos:pos+left])
					pos += left
					break
				}

				feedbuf = true
			}
		}
	}

	r.reqbuf.Next(int(pos))
	log.Println("left reqbuf len =", r.reqbuf.Len())

	return 0
}

func (t *Trunk) SerializeToBytes() (bool, []byte) {
	var buf [1600]byte
	var pos int = 0
	csid := t.basic_header.csid
	rfmt := t.basic_header.fmt

	// trunk header
	buf[pos] = byte((t.basic_header.fmt << 6))

	if csid >= 2 && csid <= 63 {
		buf[pos] = byte(int(buf[pos]) | (csid))
		pos++
	} else if csid >= 64 && csid <= 319 {
		pos++
		buf[pos] = byte(csid - 64)
		pos++
	} else if csid > 319 {
		newcsid := csid - 64
		buf[pos] = byte(int(buf[pos]) & (1 << 2))
		pos++
		binary.BigEndian.PutUint16(buf[pos:], uint16(newcsid))
		pos += 2
	}

	// trunk message header
	if 0 == rfmt || 1 == rfmt {
		// fix 16777215
		if t.message_header.timestamp > 16777215 {
			log.Println("fmt is not valid")
			return false, nil
		}

		// timestamp (3B)
		buf[pos] = byte(t.message_header.timestamp & 0xff)
		pos++
		buf[pos] = byte(t.message_header.timestamp & (0xff << 8))
		pos++
		buf[pos] = byte(t.message_header.timestamp & (0xff << 16))
		pos++

		// message length (3B)
		buf[pos] = byte(t.message_header.msglen & (0xff << 16))
		pos++
		buf[pos] = byte(t.message_header.msglen & (0xff << 8))
		pos++
		buf[pos] = byte(t.message_header.msglen & (0xff << 0))
		pos++

		// message type id (1B)
		buf[pos] = byte(t.message_header.msgtype)
		// log.Println("msg type id = ", t.message_header.msgtype)
		pos++

		if 0 == rfmt {
			// msg stream id (4B)
			binary.BigEndian.PutUint32(buf[pos:], t.message_header.msgstreamid)
			pos += 4
		}
	} else if 2 == rfmt {
		// timestamp (3B)
		buf[pos] = byte(t.message_header.timestamp & 0xff)
		pos++
		buf[pos] = byte(t.message_header.timestamp & (0xff << 8))
		pos++
		buf[pos] = byte(t.message_header.timestamp & (0xff << 16))
		pos++
	} else if 3 == rfmt {
		log.Println("no messag header")
	}

	b := append(buf[0:pos], t.payload.payload.Bytes()...)
	pos += int(t.message_header.msglen)

	// log.Println("serilized buf size", len(b), b)

	log.Println("streamid = ", t.message_header.msgstreamid)

	return true, b
}

func (r *RtmpConn) handleMessage() {
	var t *Trunk = &r.trunk
	if t.message_header.msgtype == RTMP_MSG_TYPEID_AMF0 {
		_, cmd := amf.DecodeString(&t.payload.payload)
		log.Println("the cmd is", cmd)
		switch cmd {
		case "connect":
			log.Printf("handle %s\n", cmd)
			r.handleNetConnectionConect(&t.payload.payload)
		case "createStream":
			log.Printf("handle %s\n", cmd)
			r.HandleCreateStream(&t.payload.payload)
		case "publish":
			log.Printf("handle %s\n", cmd)
			r.HandlePublish(&t.payload.payload)
		case "deleteStream":
			log.Printf("handle %s\n", cmd)
			r.HandleDeleteStream(&t.payload.payload)
		default:
			log.Printf("unknown amf cmd handing %s, ignore\n", cmd)
		}

		// server response:
		// <-- window acknowledgement size
		// <-- peer bindwidth

		// Chunk Stream ID with value 2 is
		// reserved for low-level protocol control messages and commands.
	} else if t.message_header.msgtype == RTMP_MSG_TYPEID_INVIKE { // metadata
		log.Printf("metadata msg: %s\n", spew.Sdump(t.payload.payload.Bytes()))
	} else if t.message_header.msgtype == RTMP_MSG_TYPEID_AUDIO_PKT ||
		t.message_header.msgtype == RTMP_MSG_TYPEID_VIDEO_PKT {
		// dispatch audio/video
		if _, ok := streammap[r.streamname]; !ok {
			return
		}

		ls := streammap[r.streamname]

		var b bytes.Buffer
		PackFlvTag(&b, uint8(t.message_header.msgtype), uint32(t.message_header.timestamp),
			t.payload.payload)

		if t.message_header.msgtype == RTMP_MSG_TYPEID_VIDEO_PKT {
			// save recent gop
			// the next key frame
			if t.payload.payload.Bytes()[0]&0xf0 == 0x10 {
				if ls.gopcache.Len() > 0 {
					ls.gopcache.Init()
				}
			}
			ls.gopcache.PushBack(b)
			log.Println("new msg in gop cache", ls.gopcache.Len())

			var avc_packettype uint8 = t.payload.payload.Bytes()[1]
			if avc_packettype == 0 {
				r.avc_pkt_type.Write(b.Bytes())
			}
		}

		if t.message_header.msgtype == RTMP_MSG_TYPEID_AUDIO_PKT {
			var aac_packettype uint8 = t.payload.payload.Bytes()[1]
			if aac_packettype == 0 {
				r.acc_pkt_type.Write(b.Bytes())
			}
		}

		dispatched_video := false
		for k, v := range ls.pullnodemap {
			if v.recycle {
				log.Println("recycle stream pullinfo")
				delete(ls.pullnodemap, k)
				continue
			}

			if v.registered {
				if !v.pulling {
					select {
					case v.channel <- r.acc_pkt_type:
					default:
						log.Println("channel error")
						continue
					}

					select {
					case v.channel <- r.avc_pkt_type:
					default:
						log.Println("channel error")
						continue
					}
					v.pulling = true
				}

				if t.message_header.msgtype == RTMP_MSG_TYPEID_VIDEO_PKT {
					v.gopcache.PushBack(ls.gopcache.Front().Value)

					select {
					case v.channel <- v.gopcache.Front().Value.(bytes.Buffer):
					default:
						log.Println("channel error")
						continue
					}

					v.gopcache.Remove(v.gopcache.Front())

					dispatched_video = true
				} else {
					select {
					case v.channel <- b:
					default:
						continue
					}
				}
			}
		}
		if dispatched_video {
			ls.gopcache.Remove(ls.gopcache.Front())
		}
	}
}

func (r *RtmpConn) handleNetConnectionConect(buf *bytes.Buffer) bool {
	var connect Connect
	if ret := connect.Parse(buf); !ret {
		return false
	}

	r.SendWindowAckSize()
	r.SendSetPeerBindWidth()
	r.ResponseConnect()
	return true
}

func (r *RtmpConn) SendWindowAckSize() {
	var t Trunk
	t.basic_header.fmt = 0
	t.basic_header.csid = 2 // fixed
	t.message_header.msglen = 4
	t.message_header.msgstreamid = 0 // fixed
	t.message_header.msgtype = RTMP_MSG_TYPEID_SERVER_BINDWIDTH
	// t.message_header.timestamp; ignore
	var as [4]byte
	as[0] = 0x1
	as[1] = 0x1
	as[2] = 0x1
	as[3] = 0x1
	binary.Write(&t.payload.payload, binary.LittleEndian, as)

	_, b := t.SerializeToBytes()
	r.conn.Write(b[0:])
}

func (r *RtmpConn) SendSetPeerBindWidth() {
	var t Trunk

	t.basic_header.fmt = 0
	t.basic_header.csid = 2 // fixed
	t.message_header.msglen = 5
	t.message_header.msgstreamid = 0 // fixed
	t.message_header.msgtype = RTMP_MSG_TYPEID_CLIENT_BINDWIDTH
	// t.message_header.timestamp; ignore
	var bw [5]byte
	bw[0] = 0x1
	bw[1] = 0x1
	bw[2] = 0x1
	bw[3] = 0x1
	bw[4] = 0x1
	binary.Write(&t.payload.payload, binary.LittleEndian, bw)

	_, b := t.SerializeToBytes()
	r.conn.Write(b[0:])
}

func (r *RtmpConn) ResponseConnect() {
	var cs CreateStream
	cs.cmd = "_result"
	cs.transaction_id = 1

	var rspbuf bytes.Buffer
	amf.EncodeString(&rspbuf, "_result")
	amf.EncodeNumber(&rspbuf, 1)
	log.Println(spew.Sdump(rspbuf.Bytes()))

	var t Trunk
	t.basic_header.fmt = 0
	t.basic_header.csid = 3
	t.message_header.msglen = uint32(rspbuf.Len())
	t.message_header.msgstreamid = 0
	t.message_header.msgtype = RTMP_MSG_TYPEID_AMF0
	binary.Write(&t.payload.payload, binary.LittleEndian, rspbuf.Bytes())

	_, b := t.SerializeToBytes()
	r.conn.Write(b)
}

func (r *RtmpConn) HandleCreateStream(buf *bytes.Buffer) bool {
	if r.stream_created {
		r.exit = true
		return false
	}

	r.stream_created = true

	var cs CreateStream
	if ret := cs.Parse(buf); !ret {
		return false
	}
	log.Println(spew.Sdump(cs))

	// create createStream packet which is from server to client
	// it will specify stream id
	var b bytes.Buffer
	amf.EncodeString(&b, "_result")

	var f float64 = float64(4)
	amf.EncodeNumber(&b, f)
	binary.Write(&b, binary.LittleEndian, byte(0x5))

	f = 1
	amf.EncodeNumber(&b, f)
	binary.Write(&b, binary.LittleEndian, byte(0x5))

	log.Println(spew.Sdump(b.Bytes()))

	var rspt Trunk
	rspt.basic_header.fmt = 0
	rspt.basic_header.csid = 3
	rspt.message_header.msglen = uint32(b.Len())
	rspt.message_header.msgstreamid = 0 // fixed
	rspt.message_header.msgtype = RTMP_MSG_TYPEID_AMF0
	rspt.payload.payload.Write(b.Bytes())

	_, bb := rspt.SerializeToBytes()

	log.Println("len amf create stream =", len(b.Bytes()), spew.Sdump(bb))
	r.conn.Write(bb[0:])
	return true
}

func (r *RtmpConn) HandlePublish(buf *bytes.Buffer) bool {
	// parse streamid
	log.Printf("publish payload = %s\n", spew.Sdump(buf.Bytes()))
	var pub Publish
	if ret := pub.Parse(buf); !ret {
		return false
	}
	r.streamname = pub.publishing_name

	// insert new stream info
	ls := new(LiveStream)
	ls.pullnodemap = make(map[ClientNode]*PullInfo)
	ls.gopcache.Init()
	streammap[pub.publishing_name] = ls

	var b bytes.Buffer
	amf.EncodeString(&b, "onStatus")

	var f float64 = 0
	amf.EncodeNumber(&b, f)
	binary.Write(&b, binary.LittleEndian, byte(0x5))

	// todo
	binary.Write(&b, binary.LittleEndian, byte(amf.AMF0_MARKER_OBJECT))
	binary.Write(&b, binary.BigEndian, uint16(len("code")))
	binary.Write(&b, binary.LittleEndian, []byte("code"))
	binary.Write(&b, binary.LittleEndian, byte(amf.AMF0_MARKER_STRING))
	binary.Write(&b, binary.BigEndian, uint16(len("NetStream.Play.Start")))
	binary.Write(&b, binary.LittleEndian, []byte("NetStream.Play.Start"))

	binary.Write(&b, binary.BigEndian, uint16(len("description")))
	binary.Write(&b, binary.LittleEndian, []byte("description"))
	binary.Write(&b, binary.LittleEndian, byte(amf.AMF0_MARKER_STRING))
	binary.Write(&b, binary.BigEndian, uint16(len("start publish the string")))
	binary.Write(&b, binary.LittleEndian, []byte("start publish the string"))

	binary.Write(&b, binary.LittleEndian, byte(0))
	binary.Write(&b, binary.LittleEndian, byte(0))
	binary.Write(&b, binary.LittleEndian, byte(amf.AMF0_MARKER_OBJECT_END))

	log.Println(spew.Sdump(b.Bytes()))

	var rspt Trunk
	rspt.Init()
	rspt.basic_header.fmt = 0
	rspt.basic_header.csid = 3
	rspt.message_header.msglen = uint32(len(b.Bytes()))
	rspt.message_header.msgstreamid = 0 // fixed
	rspt.message_header.msgtype = RTMP_MSG_TYPEID_AMF0
	rspt.payload.payload = *bytes.NewBuffer(b.Bytes())

	_, bb := rspt.SerializeToBytes()

	log.Printf("len(publish response)=%d|%s\n", len(b.Bytes()), spew.Sdump(bb))
	r.conn.Write(bb[0:])
	return true
}

func (r *RtmpConn) HandleDeleteStream(buf *bytes.Buffer) bool {
	var ds DeleteStream
	if ret := ds.Parse(buf); !ret {
		return false
	}

	log.Println("deleteStream:", spew.Sdump(ds))
	r.exit = true
	return true
}
