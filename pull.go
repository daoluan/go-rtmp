package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// ipv4 only
func parseIPPort(s string) (uint32, uint16) {
	ipport := strings.Split(s, ":")
	if len(ipport) == 1 {
		ip := net.ParseIP(ipport[0])
		if ip != nil {
			return binary.BigEndian.Uint32(ip), 80
		}
	} else if len(ipport) == 2 {
		ip := net.ParseIP(ipport[0])
		port, _ := strconv.Atoi(ipport[1])
		return binary.BigEndian.Uint32(ip), uint16(port)
	}

	return 0, 0
}

func pullStream(w http.ResponseWriter, r *http.Request) {
	// find the stream

	log.Printf("uri = %s\n", r.RequestURI)

	if val, ok := streammap[r.RequestURI[1:]]; ok {

		// register stream reqeust
		var cn ClientNode
		cliip, cliport := parseIPPort(r.RemoteAddr)
		cn.ip = cliip
		cn.port = cliport
		var pi *PullInfo = new(PullInfo)
		pi.registered = true
		pi.pulling = false
		pi.recycle = false
		pi.channel = make(chan bytes.Buffer, 10)
		val.pullnodemap[cn] = pi

		// return flv head
		var szpretag uint32 = 0
		bsszpretag := make([]byte, 4)

		var flvhead FLVHeader
		flvhead.flv = [3]byte{'F', 'L', 'V'}
		flvhead.ver = 0x1
		flvhead.sinfo = 0x5
		flvhead.len = 9

		// write to file for test
		// f, _ := os.Create("/tmp/3000_dylanzheng")
		// f.Write(flvhead)

		flusher, _ := w.(http.Flusher)
		_, err := w.Write([]byte(flvhead.toBytes()))
		if err != nil {
			log.Println("write error")
			return
		}

		flusher.Flush()

		// recv audio/video package from channel
		for {
			tag := <-val.pullnodemap[cn].channel

			binary.BigEndian.PutUint32(bsszpretag, szpretag)
			// f.Write(bsszpretag)
			// f.Write(tag.Bytes())

			_, err = w.Write(bsszpretag)
			if err != nil {
				log.Println("write error")
				break
			}

			_, err = w.Write(tag.Bytes())
			if err != nil {
				log.Println("write error")
				break
			}

			flusher.Flush()
			szpretag = uint32(tag.Len())
		}

		val.pullnodemap[cn].recycle = true
	} else {
		log.Println("r.remoteaddr:", r.RemoteAddr)
		http.NotFound(w, r)
	}
}

func HttpServer() {
	http.HandleFunc("/", pullStream)
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
