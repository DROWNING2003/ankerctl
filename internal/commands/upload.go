package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/screen"
)

func UploadScreen(o Options, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	input, readErr := io.ReadAll(io.LimitReader(file, screen.MaxInputBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	data, err := screen.Prepare(input)
	if err != nil {
		return err
	}
	header, crc, err := protocol.ScreenUploadHeader(data)
	if err != nil {
		return err
	}
	if crc == 0 {
		return fmt.Errorf("image CRC is zero (device treats this as empty); use another image")
	}
	tr, queue, statePath, unlock, err := openGallery(o)
	if err != nil {
		return err
	}
	defer unlock()
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	count, bitmap, err := galleryState(ctx, tr)
	if err != nil {
		return fmt.Errorf("cannot safely determine gallery capacity: %w", err)
	}
	victim, err := queue.Plan(crc, count, bitmap)
	if err != nil {
		return err
	}
	for _, entry := range queue.Entries {
		if entry.CRC == crc {
			if err := tr.Send(ctx, protocol.ScreenSelectByCRC(crc)); err != nil {
				return err
			}
			if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
				return err
			}
			actual, err := currentScreen(ctx, tr)
			if err != nil || actual == nil || actual.CRC != crc || actual.Slot != entry.Slot {
				return fmt.Errorf("existing wallpaper could not be confirmed; refusing duplicate upload")
			}
			if o.JSON {
				return writeJSON(o.Out, map[string]any{"status": "reused", "crc16": fmt.Sprintf("%04x", crc)})
			}
			fmt.Fprintf(o.Out, "existing wallpaper selected: crc16=0x%04x (no flash upload)\n", crc)
			return nil
		}
	}
	if victim != nil {
		if err := evictOldest(ctx, tr, *victim); err != nil {
			return err
		}
		kept := queue.Entries[:0]
		for _, entry := range queue.Entries {
			if entry.CRC != victim.CRC {
				kept = append(kept, entry)
			}
		}
		queue.Entries = kept
		if err := queue.Save(statePath); err != nil {
			return fmt.Errorf("oldest image deleted but queue save failed: %w", err)
		}
		if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
			return err
		}
	}
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
	if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	actual, err := currentScreen(ctx, tr)
	if err != nil || actual == nil || actual.CRC != crc || actual.Slot > 7 {
		return fmt.Errorf("uploaded image activation not confirmed; queue not updated (read error: %v)", err)
	}
	queue.Record(crc, actual.Slot, 0)
	if err := queue.Save(statePath); err != nil {
		return fmt.Errorf("image active but queue save failed: %w", err)
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
