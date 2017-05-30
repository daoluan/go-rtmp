package amf

import (
	"bytes"
	"encoding/binary"
)

const (
	AMF0_MARKER_NUMBER       = 0X00
	AMF0_MARKER_BOOL         = 0X01
	AMF0_MARKER_STRING       = 0X02
	AMF0_MARKER_OBJECT       = 0X03
	AMF0_MARKER_NULL         = 0X05
	AMF0_MARKER_ECMA_ARRAY   = 0X08
	AMF0_MARKER_OBJECT_END   = 0X09
	AMF0_MARKER_STRICT_ARRAY = 0X0a
	AMF0_MARKER_DATE         = 0X0b
	AMF0_MARKER_LONG_STRING  = 0X0c
	AMF0_MARKER_XML          = 0X0f
	AMF0_MARKER_TYPED_OBJECT = 0X10
	AMF0_MARKER_AMF3         = 0X11
)

func EncodeByte(buf *bytes.Buffer, b byte) {
	binary.Write(buf, binary.LittleEndian, b)
}

func EncodeString(buf *bytes.Buffer, s string) {
	EncodeByte(buf, byte(AMF0_MARKER_STRING))
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	binary.Write(buf, binary.LittleEndian, []byte(s))
}

func EncodeNumber(buf *bytes.Buffer, f float64) {
	EncodeByte(buf, byte(AMF0_MARKER_NUMBER))
	binary.Write(buf, binary.BigEndian, f)
}

func DecodeBytes(buf *bytes.Buffer, n int) (bool, []byte) {
	return true, buf.Next(n)
}

func DecodeString(buf *bytes.Buffer) (bool, string) {
	// skip marker
	buf.Next(1)
	var strlen uint16 = binary.BigEndian.Uint16(buf.Next(2))
	str := buf.Next(int(strlen))

	return true, string(str)
}

func DecodeObjectKey(buf *bytes.Buffer) (bool, string) {
	// skip marker
	var strlen uint16 = binary.BigEndian.Uint16(buf.Next(2))
	// the end of object
	if strlen == 0 {
		return false, ""
	}
	str := buf.Next(int(strlen))

	return true, string(str)
}

func DecodeNumber(buf *bytes.Buffer) (bool, float64) {
	buf.Next(1)
	var f float64 = 0
	binary.Read(buf, binary.BigEndian, &f)

	return true, f
}

func DecodeBool(buf *bytes.Buffer) (bool, bool) {
	buf.Next(1)
	var b bool = false
	binary.Read(buf, binary.LittleEndian, &b)
	return true, b
}
