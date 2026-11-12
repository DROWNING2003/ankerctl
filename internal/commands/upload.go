package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"time"

	"ankerctl/internal/protocol"
)

func UploadScreen(o Options, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return fmt.Errorf("screen upload requires a JPEG file")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid JPEG: %w", err)
	}
	if cfg.Width != 320 || cfg.Height != 240 {
		return fmt.Errorf("screen image must be 320x240, got %dx%d", cfg.Width, cfg.Height)
	}
	header, crc, err := protocol.ScreenUploadHeader(data)
	if err != nil {
		return err
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	if resp, err := tr.Request(ctx, header); err != nil {
		return err
	} else if status, err := protocol.ParseScreenStatus(resp, 0x20); err != nil || status != 0 {
		return fmt.Errorf("screen upload handshake rejected: status=%d err=%v", status, err)
	}
	for seq, off := uint32(0), 0; off < len(data); seq, off = seq+1, off+protocol.ScreenChunkSize {
		end := off + protocol.ScreenChunkSize
		if end > len(data) {
			end = len(data)
		}
		pkt, _ := protocol.ScreenDataPacket(seq, data[off:end])
		if err := tr.Send(ctx, pkt); err != nil {
			return err
		}
		time.Sleep(7 * time.Millisecond)
	}
	resp, err := tr.Request(ctx, nil)
	if err != nil {
		return err
	}
	status, err := protocol.ParseScreenStatus(resp, 0x21)
	if err != nil || status != 0x10 {
		return fmt.Errorf("screen upload completion rejected: status=%d err=%v", status, err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := tr.Send(ctx, protocol.ScreenSelectByCRC(crc)); err != nil {
		return err
	}
	if o.JSON {
		return writeJSON(o.Out, map[string]any{"operation": "screen upload", "path": path, "bytes": len(data), "crc16": fmt.Sprintf("%04x", crc), "status": "ok"})
	}
	fmt.Fprintf(o.Out, "screen uploaded and activated: %s (%d bytes, crc16=0x%04x)\n", path, len(data), crc)
	return nil
}

func writeJSON(w interface{ Write([]byte) (int, error) }, v any) error {
	return json.NewEncoder(w).Encode(v)
}
