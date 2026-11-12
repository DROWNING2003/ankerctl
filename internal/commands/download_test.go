package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"image"
	"image/jpeg"
	"testing"
)

type downloadMock struct {
	data     []byte
	wrongSeq bool
	calls    int
}

func (m *downloadMock) RequestSized(ctx context.Context, tx []byte, prefix [2]byte, length int) ([]byte, error) {
	m.calls++
	if tx[0] == 3 {
		return []byte{11, 101, 0}, nil
	}
	if tx[2] == 0 {
		f := make([]byte, 11)
		f[0] = 10
		f[1] = 101
		binary.BigEndian.PutUint32(f[3:], uint32(len(m.data)))
		binary.BigEndian.PutUint32(f[7:], uint32((len(m.data)+56)/57))
		return f, nil
	}
	seq := binary.BigEndian.Uint32(tx[3:])
	offset := int(seq) * 57
	f := make([]byte, 7)
	f[0] = 10
	f[1] = 101
	if m.wrongSeq {
		seq++
	}
	binary.BigEndian.PutUint32(f[3:], seq)
	return append(f, m.data[offset:min(offset+57, len(m.data))]...), nil
}
func TestDownloadReadsImageAndCRC(t *testing.T) {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 320, 240)), nil)
	m := &downloadMock{data: b.Bytes()}
	got, err := readWallpaper(context.Background(), m, protocol.CRC16IBM(m.data))
	if err != nil || !bytes.Equal(got, m.data) {
		t.Fatalf("%v %d", err, len(got))
	}
	m.wrongSeq = true
	if _, err := readWallpaper(context.Background(), m, protocol.CRC16IBM(m.data)); err == nil {
		t.Fatal("accepted wrong sequence")
	}
}
