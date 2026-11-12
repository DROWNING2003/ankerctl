package commands

import (
	"context"
	"encoding/binary"
	"testing"
)

type invalidDownload struct{ kind string }

func (m invalidDownload) RequestSized(_ context.Context, tx []byte, _ [2]byte, _ int) ([]byte, error) {
	if tx[0] == 3 {
		if m.kind == "busy" {
			return []byte{11, 101, 3}, nil
		}
		return []byte{11, 101, 0}, nil
	}
	f := make([]byte, 11)
	f[0] = 10
	f[1] = 101
	binary.BigEndian.PutUint32(f[3:], 65521)
	binary.BigEndian.PutUint32(f[7:], 1)
	return f, nil
}
func TestDownloadRejectsBusyAndOversize(t *testing.T) {
	for _, kind := range []string{"busy", "size"} {
		if _, err := readWallpaper(context.Background(), invalidDownload{kind}, 1); err == nil {
			t.Fatal("accepted " + kind)
		}
	}
}
