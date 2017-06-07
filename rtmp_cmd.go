package main

import (
	"bytes"
	"go_rtmp_srv/amf"
)

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
		var bret bool = false
		switch field {
		case "app":
			bret, c.app = amf.DecodeString(buf)
			if !bret {
				return false
			}
		case "flashver":
			bret, c.flashver = amf.DecodeString(buf)
			if !bret {
				return false
			}
		case "swfUrl":
			bret, c.swfurl = amf.DecodeString(buf)
			if !bret {
				return false
			}
		case "tcUrl":
			bret, c.tcurl = amf.DecodeString(buf)
			if !bret {
				return false
			}
		case "fpad":
			bret, c.fpad = amf.DecodeBool(buf)
			if !bret {
				return false
			}
		case "audioCodecs":
			bret, f = amf.DecodeNumber(buf)
			if !bret {
				return false
			}
			c.audiocodecs = int(f)
		case "videoCodecs":
			bret, f = amf.DecodeNumber(buf)
			if !bret {
				return false
			}
			c.videocodecs = int(f)
		case "videoFunction":
			bret, f = amf.DecodeNumber(buf)
			if !bret {
				return false
			}
			c.videofunc = int(f)
		case "pageUrl":
			bret, c.pageurl = amf.DecodeString(buf)
			if !bret {
				return false
			}
		case "type":
			bret, c.ttype = amf.DecodeString(buf)
			if !bret {
				return false
			}
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
