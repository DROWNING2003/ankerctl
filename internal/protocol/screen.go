package protocol

import (
	"encoding/binary"
	"fmt"
)

const MaxJPEGSize = 65520
const ScreenChunkSize = 55

func ScreenUploadHeader(image []byte) ([]byte, uint16, error) {
	if len(image) == 0 || len(image) > MaxJPEGSize {
		return nil, 0, fmt.Errorf("JPEG size must be 1..%d bytes", MaxJPEGSize)
	}
	crc := CRC16IBM(image)
	packets := (len(image) + ScreenChunkSize - 1) / ScreenChunkSize
	h := make([]byte, 32)
	copy(h, []byte{2, 32, 161, 1, 255, 162, 5, 3})
	binary.BigEndian.PutUint32(h[8:], uint32(len(image)))
	h[12] = 163
	h[13] = 2
	h[14] = 1
	h[15] = 0
	h[16] = 164
	h[17] = 3
	h[18] = 2
	binary.BigEndian.PutUint16(h[19:], ScreenChunkSize)
	h[21] = 165
	h[22] = 5
	h[23] = 3
	binary.BigEndian.PutUint32(h[24:], uint32(packets))
	h[28] = 166
	h[29] = 2
	binary.BigEndian.PutUint16(h[30:], crc)
	return h, crc, nil
}
func ScreenDataPacket(seq uint32, payload []byte) ([]byte, error) {
	if len(payload) > ScreenChunkSize {
		return nil, fmt.Errorf("payload exceeds %d bytes", ScreenChunkSize)
	}
	p := make([]byte, 6+len(payload))
	p[0] = 2
	p[1] = 33
	binary.BigEndian.PutUint32(p[2:], seq)
	copy(p[6:], payload)
	return p, nil
}
func ScreenSelectByCRC(crc uint16) []byte {
	p := []byte{2, 25, 0, 0}
	binary.BigEndian.PutUint16(p[2:], crc)
	return p
}
func ScreenCurrentRequest() []byte { return []byte{2, 24} }
func ParseScreenStatus(frame []byte, want byte) (byte, error) {
	if len(frame) < 3 || frame[0] != 0x0a || frame[1] != want {
		return 0, fmt.Errorf("unexpected response")
	}
	return frame[2], nil
}
