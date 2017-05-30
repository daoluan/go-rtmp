package main

import (
	"bytes"
	"container/list"
	"log"
	"net"
	"os"
)

type ClientNode struct {
	ip   uint32
	port uint16
}

type PullInfo struct {
	registered bool
	channel    chan bytes.Buffer
	pulling    bool // pull action has began
	gopcache   list.List
	recycle    bool // recycle flag
}

type LiveStream struct {
	gopcache    list.List
	pullnodemap map[ClientNode]*PullInfo
}

// stream map: streamid to stream info
var streammap = map[string]*LiveStream{}
var codecinfo_audio = new(bytes.Buffer)
var codecinfo_video = new(bytes.Buffer)

func (r *RtmpServer) Run(rtmp_conf RtmpConf) bool {
	l, err := net.Listen("tcp", rtmp_conf.server_addr.String())
	if err != nil {
		log.Println("fail to listen", err, rtmp_conf.server_addr.String())
		return false
	}

	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("fail to accept: ", err.Error())
			return false
		}

		go HandleNewConnection(conn)
	}

	return true
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	f, err := os.OpenFile("rtmp.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	var rtmp_conf RtmpConf
	rtmp_conf.server_addr.IP = net.ParseIP("0.0.0.0")
	rtmp_conf.server_addr.Port = 1935
	log.Println(rtmp_conf)

	go HttpServer()

	var srv RtmpServer
	srv.Run(rtmp_conf)
}
