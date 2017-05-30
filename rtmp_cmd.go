package main

import (
	"bytes"
	"go_rtmp_srv/amf"
)

type ConnCmdObj struct {
	App         string
	Type        string
	Flashver    string
	Swfurl      string
	Tcurl       string
	Fpad        bool
	Audiocodecs int
	Videocodecs int
	Videofunc   int
	Pageurl     string
}

type Connect struct {
	cmd            string
	transaction_id uint64

	app         string
	ttype       string // OH
	flashver    string
	swfurl      string
	tcurl       string
	fpad        bool
	audiocodecs int
	videocodecs int
	videofunc   int
	pageurl     string
}

func (c *Connect) Parse(buf *bytes.Buffer) bool {
	c.cmd = "connect"

	_, tid := amf.DecodeNumber(buf)
	c.transaction_id = uint64(tid)
	if buf.Len() == 0 {
		return true
	}

	buf.Next(1)

	// parser cmd object
	for {
		ret, field := amf.DecodeObjectKey(buf)
		if ret == false {
			break
		}

		var f float64 = 0
		switch field {
		case "app":
			_, c.app = amf.DecodeString(buf)
		case "flashver":
			_, c.flashver = amf.DecodeString(buf)
		case "swfUrl":
			_, c.swfurl = amf.DecodeString(buf)
		case "tcUrl":
			_, c.tcurl = amf.DecodeString(buf)
		case "fpad":
			_, c.fpad = amf.DecodeBool(buf)
		case "audioCodecs":
			_, f = amf.DecodeNumber(buf)
			c.audiocodecs = int(f)
		case "videoCodecs":
			_, f = amf.DecodeNumber(buf)
			c.videocodecs = int(f)
		case "videoFunction":
			_, f = amf.DecodeNumber(buf)
			c.videofunc = int(f)
		case "pageUrl":
			_, c.pageurl = amf.DecodeString(buf)
		case "type":
			_, c.ttype = amf.DecodeString(buf)
		default:
			// skip this field
			t := buf.Next(1)[0]
			if t == amf.AMF0_MARKER_NUMBER {
				amf.DecodeNumber(buf)
			} else if t == amf.AMF0_MARKER_BOOL {
				amf.DecodeBool(buf)
			} else if t == amf.AMF0_MARKER_STRING {
				amf.DecodeString(buf)
			}
		}
	}

	return true
}

type CreateStream struct {
	cmd            string
	transaction_id uint64
}

func (c *CreateStream) Parse(buf *bytes.Buffer) bool {
	c.cmd = "createStream"

	_, tid := amf.DecodeNumber(buf)
	c.transaction_id = uint64(tid)
	if buf.Len() == 0 {
		return true
	}

	f := buf.Next(1)[0]

	if f == amf.AMF0_MARKER_OBJECT {
		// parse object
	}

	return true
}

type Publish struct {
	cmd             string
	transaction_id  uint64
	publishing_name string
	publishing_type string
}

func (p *Publish) Parse(buf *bytes.Buffer) bool {
	p.cmd = "publish"

	_, tid := amf.DecodeNumber(buf)
	p.transaction_id = uint64(tid)
	buf.Next(1) // null object

	_, p.publishing_name = amf.DecodeString(buf)
	_, p.publishing_type = amf.DecodeString(buf)
	return true
}

type Play struct {
	cmd            string
	transaction_id uint64
	streamname     string
}

func (p *Play) Parse(buf *bytes.Buffer) bool {
	p.cmd = "play"

	_, tid := amf.DecodeNumber(buf)
	p.transaction_id = uint64(tid)
	if buf.Len() == 0 {
		return true
	}

	buf.Next(1) // null object

	_, p.streamname = amf.DecodeString(buf)

	// ignore start, duration, reset fields
	return true
}

type DeleteStream struct {
	cmd            string
	transaction_id uint64
	streamid       uint64
}

func (d *DeleteStream) Parse(buf *bytes.Buffer) bool {
	d.cmd = "deleteStream"

	_, tid := amf.DecodeNumber(buf)
	d.transaction_id = uint64(tid)
	if buf.Len() == 0 {
		return true
	}

	buf.Next(1) // null object

	_, streamid := amf.DecodeNumber(buf)
	d.streamid = uint64(streamid)

	return true
}
