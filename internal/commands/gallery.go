package commands

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/device"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/screen"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

// Queue state is scoped to the selected OS port, never a device serial number.
// If a port changes, the empty new queue will not evict unknown existing images.
func openGallery(o Options) (serial.Transport, *screen.Queue, string, func(), error) {
	path := o.Port
	if path == "" {
		ports, err := device.ListPorts()
		if err != nil {
			return nil, nil, "", nil, err
		}
		for _, p := range ports {
			if p.IsA210C {
				if path != "" {
					return nil, nil, "", nil, fmt.Errorf("multiple A210C devices; select --port")
				}
				path = p.Path
			}
		}
	}
	if path == "" {
		return nil, nil, "", nil, fmt.Errorf("no A210C found")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, nil, "", nil, err
	}
	dir := filepath.Join(config, "ankerctl", "gallery")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, nil, "", nil, err
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))
	statePath := filepath.Join(dir, key+".json")
	lockPath := statePath + ".lock"
	// Exclusive creation, not a lease: stale locks fail closed after a crash.
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("gallery is locked (or stale lock at %s): %w", lockPath, err)
	}
	_ = lock.Close()
	unlock := func() { _ = os.Remove(lockPath) }
	queue, err := screen.LoadQueue(statePath)
	if err != nil {
		unlock()
		return nil, nil, "", nil, err
	}
	o.Port = path
	tr, err := Open(o)
	if err != nil {
		unlock()
		return nil, nil, "", nil, err
	}
	return tr, queue, statePath, unlock, nil
}
func galleryState(ctx context.Context, tr serial.Transport) (uint8, uint8, error) {
	f, err := tr.Request(ctx, []byte{2, 0x1c})
	if err != nil {
		return 0, 0, err
	}
	if len(f) != 8 || f[0] != 10 || f[1] != 0x1c {
		return 0, 0, fmt.Errorf("invalid gallery state response")
	}
	return f[3], f[5], nil
}
func currentScreen(ctx context.Context, tr serial.Transport) (*protocol.CurrentScreen, error) {
	f, err := tr.Request(ctx, protocol.ScreenCurrentRequest())
	if err != nil {
		return nil, err
	}
	return protocol.ParseCurrentScreen(f)
}
func waitGallery(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func evictOldest(ctx context.Context, tr serial.Transport, entry screen.QueueEntry) error {
	// Check CRC existence through activation + readback before any deletion.
	previous, err := currentScreen(ctx, tr)
	if err != nil {
		return err
	}
	if err := tr.Send(ctx, protocol.ScreenSelectByCRC(entry.CRC)); err != nil {
		return err
	}
	if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	actual, err := currentScreen(ctx, tr)
	if err != nil || actual == nil || actual.CRC != entry.CRC || actual.Slot != entry.Slot {
		if previous != nil {
			_ = tr.Send(ctx, protocol.ScreenSelectByCRC(previous.CRC))
		}
		return fmt.Errorf("oldest image could not be verified; automatic deletion refused (read error: %v)", err)
	}
	packet := []byte{2, 0x1a, 0, 0}
	binary.BigEndian.PutUint16(packet[2:], entry.CRC)
	f, err := tr.Request(ctx, packet)
	if err != nil {
		return fmt.Errorf("deletion result unknown; inspect gallery before retrying: %w", err)
	}
	status, err := protocol.ParseScreenStatus(f, 0x1a)
	if err != nil || len(f) != 3 || status != 0 {
		return fmt.Errorf("oldest image deletion not confirmed: status=%d error=%v", status, err)
	}
	return nil
}
