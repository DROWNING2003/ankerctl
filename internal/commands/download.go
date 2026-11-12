package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image/jpeg"
	"os"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
)

type sizedRequester interface {
	RequestSized(context.Context, []byte, [2]byte, int) ([]byte, error)
}

func readWallpaper(ctx context.Context, tr sizedRequester, wantCRC uint16) ([]byte, error) {
	handshake := make([]byte, 64)
	handshake[0] = 3
	handshake[1] = 101
	f, err := tr.RequestSized(ctx, handshake, [2]byte{11, 101}, 3)
	if err != nil {
		return nil, err
	}
	if len(f) != 3 || f[0] != 11 || f[1] != 101 || f[2] != 0 {
		return nil, fmt.Errorf("wallpaper read handshake busy/rejected")
	}
	info := make([]byte, 64)
	info[0] = 2
	info[1] = 101
	f, err = tr.RequestSized(ctx, info, [2]byte{10, 101}, 11)
	if err != nil {
		return nil, err
	}
	if len(f) != 11 || f[0] != 10 || f[1] != 101 {
		return nil, fmt.Errorf("invalid wallpaper info")
	}
	if f[2] != 0 {
		return nil, fmt.Errorf("wallpaper info status=%d (empty/busy/error)", f[2])
	}
	size, packets := int(binary.BigEndian.Uint32(f[3:])), int(binary.BigEndian.Uint32(f[7:]))
	if size <= 0 || size > protocol.MaxJPEGSize || packets != (size+56)/57 {
		return nil, fmt.Errorf("invalid wallpaper size/packet count")
	}
	data := make([]byte, 0, size)
	for seq := range packets {
		request := make([]byte, 64)
		request[0] = 2
		request[1] = 101
		request[2] = 1
		binary.BigEndian.PutUint32(request[3:], uint32(seq))
		payload := min(57, size-len(data))
		f, err = tr.RequestSized(ctx, request, [2]byte{10, 101}, 7+payload)
		if err != nil {
			return nil, err
		}
		if len(f) != 7+payload || f[0] != 10 || f[1] != 101 || f[2] != 0 || binary.BigEndian.Uint32(f[3:]) != uint32(seq) {
			return nil, fmt.Errorf("wallpaper packet %d malformed/rejected", seq)
		}
		data = append(data, f[7:]...)
	}
	if protocol.CRC16IBM(data) != wantCRC {
		return nil, fmt.Errorf("downloaded wallpaper CRC differs from current image; no output written")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 320 || cfg.Height != 240 {
		return nil, fmt.Errorf("downloaded image is not a valid 320x240 JPEG")
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("downloaded JPEG truncated: %w", err)
	}
	return data, nil
}
func DownloadScreen(ctx context.Context, o Options, path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	sized, ok := tr.(sizedRequester)
	if !ok {
		return protocol.Unsupported("wallpaper download transport")
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	current, err := currentScreen(ctx, tr)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("device has no current wallpaper")
	}
	if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	data, err := readWallpaper(ctx, sized, current.CRC)
	if err != nil {
		return err
	}
	after, err := currentScreen(ctx, tr)
	if err != nil {
		return err
	}
	if after == nil || after.CRC != current.CRC || after.Slot != current.Slot {
		return fmt.Errorf("wallpaper changed during download; no output written")
	}
	// Exclusive create refuses overwrites, including a file appearing during transfer.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("output write failed (partial file may exist): %w", writeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	result := map[string]any{"path": path, "bytes": len(data), "crc16": fmt.Sprintf("%04x", current.CRC), "slot": current.Slot}
	if o.JSON {
		return writeJSON(o.Out, result)
	}
	_, err = fmt.Fprintf(o.Out, "wallpaper downloaded: %s (%d bytes, CRC=%04x)\n", path, len(data), current.CRC)
	return err
}
