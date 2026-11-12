package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/screen"
	"github.com/DROWNING2003/ankerctl/internal/serial"
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
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	tr, q, state, unlock, err := openGallery(o)
	if err != nil {
		return err
	}
	defer unlock()
	defer tr.Close()
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
	defer cancel()
	result, err := runUpload(ctx, tr, q, func() error { return q.Save(state) }, data)
	if err != nil {
		return err
	}
	if o.JSON {
		return writeJSON(o.Out, result)
	}
	_, err = fmt.Fprintf(o.Out, "wallpaper %s: %d bytes crc16=%s\n", result.Status, len(data), result.CRC)
	return err
}

type uploadResult struct {
	Status string `json:"status"`
	CRC    string `json:"crc16"`
	Bytes  int    `json:"bytes"`
}

func imageEntry(data []byte) screen.QueueEntry {
	return screen.QueueEntry{CRC: protocol.CRC16IBM(data), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), JPEG: append([]byte(nil), data...)}
}
func selectImage(ctx context.Context, tr serial.Transport, entry screen.QueueEntry) (*protocol.CurrentScreen, error) {
	if err := tr.Send(ctx, protocol.ScreenSelectByCRC(entry.CRC)); err != nil {
		return nil, err
	}
	if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
		return nil, err
	}
	current, err := currentScreen(ctx, tr)
	if err != nil {
		return nil, err
	}
	return current, nil
}
func sendImage(ctx context.Context, tr serial.Transport, data []byte) error {
	header, _, err := protocol.ScreenUploadHeader(data)
	if err != nil {
		return err
	}
	response, err := tr.Request(ctx, header)
	if err != nil {
		return err
	}
	status, err := protocol.ParseScreenStatus(response, 0x20)
	if err != nil || status != 0 {
		return fmt.Errorf("upload handshake rejected: status=%d error=%v", status, err)
	}
	for seq, offset := uint32(0), 0; offset < len(data); seq, offset = seq+1, offset+protocol.ScreenChunkSize {
		packet, err := protocol.ScreenDataPacket(seq, data[offset:min(offset+protocol.ScreenChunkSize, len(data))])
		if err != nil {
			return err
		}
		if err := tr.Send(ctx, packet); err != nil {
			return err
		}
		if err := waitGallery(ctx, 7*time.Millisecond); err != nil {
			return err
		}
	}
	response, err = tr.Request(ctx, nil)
	if err != nil {
		return err
	}
	status, err = protocol.ParseScreenStatus(response, 0x21)
	if err != nil || status != 0x10 {
		return fmt.Errorf("upload completion not confirmed: status=%d error=%v", status, err)
	}
	return waitGallery(ctx, 300*time.Millisecond)
}

// Reconcile is conservative: only a positively selected CRC can complete a pending
// target. Otherwise restore a backed-up victim when a free slot proves capacity.
func recoverUpload(ctx context.Context, tr serial.Transport, q *screen.Queue, save func() error) error {
	pending := q.Pending
	if pending == nil {
		return nil
	}
	current, err := selectImage(ctx, tr, pending.Target)
	if err != nil {
		return fmt.Errorf("pending upload outcome unknown; retry upload to recover: %w", err)
	}
	if current != nil && current.CRC == pending.Target.CRC {
		pending.Target.Slot = current.Slot
		q.RecordImage(pending.Target)
		if pending.Victim != nil {
			q.Remove(pending.Victim.CRC)
		}
		q.Pending = nil
		if err := save(); err != nil {
			q.Pending = pending
			return err
		}
		return nil
	}
	count, bitmap, err := galleryState(ctx, tr)
	if err != nil {
		return err
	}
	if err := validateGallery(count, bitmap); err != nil {
		return err
	}
	if pending.Victim != nil {
		victim := *pending.Victim
		old, err := selectImage(ctx, tr, victim)
		if err != nil {
			return err
		}
		if old == nil || old.CRC != victim.CRC {
			if count >= 8 {
				return fmt.Errorf("pending FIFO recovery blocked: gallery full and victim missing; no unknown image deleted")
			}
			if err := sendImage(ctx, tr, victim.JPEG); err != nil {
				return fmt.Errorf("old wallpaper %04x needs restoration; backup retained: %w", victim.CRC, err)
			}
			old, err = selectImage(ctx, tr, victim)
			if err != nil || old == nil || old.CRC != victim.CRC {
				return fmt.Errorf("restoration not confirmed; backup retained")
			}
		}
		victim.Slot = old.Slot
		q.RecordImage(victim)
	}
	q.Pending = nil
	if err := save(); err != nil {
		q.Pending = pending
		return err
	}
	return nil
}
func runUpload(ctx context.Context, tr serial.Transport, q *screen.Queue, save func() error, data []byte) (uploadResult, error) {
	target := imageEntry(data)
	result := uploadResult{CRC: fmt.Sprintf("%04x", target.CRC), Bytes: len(data)}
	if target.CRC == 0 {
		return result, fmt.Errorf("zero image CRC is reserved")
	}
	if err := recoverUpload(ctx, tr, q, save); err != nil {
		return result, err
	}
	count, bitmap, err := galleryState(ctx, tr)
	if err != nil {
		return result, err
	}
	if err := validateGallery(count, bitmap); err != nil {
		return result, err
	}
	// Slots known empty invalidate stale entries, including externally deleted images.
	kept := q.Entries[:0]
	for _, e := range q.Entries {
		if bitmap&(1<<e.Slot) != 0 {
			kept = append(kept, e)
		}
	}
	q.Entries = kept
	if err := q.CheckIdentity(target.CRC, target.SHA256); err != nil {
		return result, err
	}
	for _, entry := range q.Entries {
		if entry.CRC == target.CRC {
			current, err := selectImage(ctx, tr, target)
			if err != nil {
				return result, err
			}
			if current != nil && current.CRC == target.CRC {
				entry.Slot = current.Slot
				q.RecordImage(entry)
				if err := save(); err != nil {
					return result, err
				}
				result.Status = "reused"
				return result, nil
			}
			// A valid current-screen response with a different CRC proves selection did not occur.
			q.Remove(target.CRC)
			if err := save(); err != nil {
				return result, err
			}
			break
		}
	}
	victim, err := q.Plan(target.CRC, count, bitmap)
	if err != nil {
		return result, err
	}
	if victim != nil {
		if len(victim.JPEG) == 0 || victim.SHA256 == "" {
			return result, fmt.Errorf("oldest image has no verified backup (legacy queue); automatic eviction refused")
		}
		verified := imageEntry(victim.JPEG)
		if verified.CRC != victim.CRC || verified.SHA256 != victim.SHA256 {
			return result, fmt.Errorf("oldest image backup integrity mismatch")
		}
	}
	if victim != nil {
		copyVictim := *victim
		copyVictim.JPEG = append([]byte(nil), victim.JPEG...)
		victim = &copyVictim
	}
	pending := &screen.PendingUpload{Target: target, Victim: victim, Phase: "prepared"}
	q.Pending = pending
	if err := save(); err != nil {
		return result, fmt.Errorf("cannot journal upload; device unchanged: %w", err)
	}
	if victim != nil {
		if err := evictOldest(ctx, tr, *victim); err != nil {
			return result, fmt.Errorf("FIFO deletion outcome uncertain; recovery journal retained: %w", err)
		}
	}
	if err := sendImage(ctx, tr, data); err != nil {
		if victim != nil && ctx.Err() == nil {
			if recoveryErr := recoverUpload(ctx, tr, q, save); recoveryErr == nil {
				return result, fmt.Errorf("upload failed; device reconciled/old wallpaper restored: %w", err)
			}
		}
		return result, fmt.Errorf("upload failed; pending image/backup retained; retry upload to reconcile or restore: %w", err)
	}
	current, err := selectImage(ctx, tr, target)
	if err != nil || current == nil || current.CRC != target.CRC {
		return result, fmt.Errorf("activation not confirmed; journal retained for retry: %v", err)
	}
	target.Slot = current.Slot
	if victim != nil {
		q.Remove(victim.CRC)
	}
	q.RecordImage(target)
	q.Pending = nil
	if err := save(); err != nil {
		q.Pending = pending
		return result, fmt.Errorf("image uploaded but queue commit failed; journal retained: %w", err)
	}
	result.Status = "uploaded"
	return result, nil
}
func writeJSON(w io.Writer, v any) error { return json.NewEncoder(w).Encode(v) }
