all: rtmp

rtmp:
	rm ../../bin/go_rtmp_srv -f
	go install 
