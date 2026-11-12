package screen

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"math/bits"
	"os"
	"path/filepath"
)

type QueueEntry struct {
	CRC    uint16 `json:"crc16"`
	Slot   uint8  `json:"slot"`
	SHA256 string `json:"sha256,omitempty"`
	JPEG   []byte `json:"jpeg,omitempty"`
}
type PendingUpload struct {
	Target QueueEntry  `json:"target"`
	Victim *QueueEntry `json:"victim,omitempty"`
	Phase  string      `json:"phase"`
}
type Queue struct {
	Entries []QueueEntry   `json:"entries"`
	Pending *PendingUpload `json:"pending,omitempty"`
}

func (q *Queue) CheckIdentity(crc uint16, hash string) error {
	for _, e := range q.Entries {
		if e.CRC == crc && e.SHA256 != hash {
			return fmt.Errorf("CRC %04x conflicts with another or legacy image; select/delete explicitly before uploading", crc)
		}
	}
	return nil
}
func (q *Queue) Remove(crc uint16) {
	kept := q.Entries[:0]
	for _, e := range q.Entries {
		if e.CRC != crc {
			kept = append(kept, e)
		}
	}
	q.Entries = kept
}
func (q *Queue) RecordImage(entry QueueEntry) {
	q.Record(entry.CRC, entry.Slot, 0)
	for i := range q.Entries {
		if q.Entries[i].CRC == entry.CRC {
			q.Entries[i] = entry
		}
	}
}

// Plan never infers upload age from slot order. Full untracked galleries fail closed.
func (q *Queue) Plan(crc uint16, count, bitmap uint8) (*QueueEntry, error) {
	if int(count) != bits.OnesCount8(bitmap) {
		return nil, fmt.Errorf("gallery count/bitmap mismatch; refusing writes")
	}
	if count > 8 {
		return nil, fmt.Errorf("invalid device gallery count %d", count)
	}
	for _, e := range q.Entries {
		if e.CRC == crc {
			return nil, nil
		}
	}
	if count < 8 {
		return nil, nil
	}
	if len(q.Entries) != 8 {
		return nil, fmt.Errorf("gallery full: complete upload order unknown; refusing to delete from a partially tracked gallery")
	}
	var tracked uint8
	for _, entry := range q.Entries {
		if entry.Slot > 7 || tracked&(1<<entry.Slot) != 0 {
			return nil, fmt.Errorf("queue slots invalid")
		}
		tracked |= 1 << entry.Slot
	}
	if tracked != bitmap {
		return nil, fmt.Errorf("queue differs from gallery")
	}
	oldest := q.Entries[0]
	if oldest.Slot > 7 || bitmap&(1<<oldest.Slot) == 0 {
		return nil, fmt.Errorf("queue differs from device gallery; refusing automatic deletion")
	}
	return &oldest, nil
}
func (q *Queue) Record(crc uint16, slot uint8, evicted uint16) {
	entries := make([]QueueEntry, 0, 8)
	found := false
	for _, e := range q.Entries {
		if e.CRC == evicted && evicted != 0 {
			continue
		}
		if e.CRC == crc {
			e.Slot = slot
			found = true
		} else if e.Slot == slot {
			continue
		}
		entries = append(entries, e)
	}
	if !found {
		entries = append(entries, QueueEntry{CRC: crc, Slot: slot})
	}
	q.Entries = entries
}
func LoadQueue(path string) (*Queue, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &Queue{}, nil
	}
	if err != nil {
		return nil, err
	}
	if stat.Size() > 2*1024*1024 {
		return nil, fmt.Errorf("queue exceeds 2 MiB")
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Queue{}, nil
	}
	if err != nil {
		return nil, err
	}
	var q Queue
	if err := json.Unmarshal(b, &q); err != nil {
		return nil, fmt.Errorf("invalid queue: %w", err)
	}
	if len(q.Entries) > 8 {
		return nil, fmt.Errorf("invalid queue length")
	}
	seenCRC := map[uint16]bool{}
	seenSlot := map[uint8]bool{}
	for _, e := range q.Entries {
		if e.CRC == 0 || e.Slot > 7 || seenCRC[e.CRC] || seenSlot[e.Slot] {
			return nil, fmt.Errorf("invalid queue entries")
		}
		seenCRC[e.CRC] = true
		seenSlot[e.Slot] = true
	}
	if q.Pending != nil {
		if q.Pending.Phase != "prepared" {
			return nil, fmt.Errorf("unknown pending phase")
		}
		if err := validateBackup(q.Pending.Target); err != nil {
			return nil, err
		}
		if q.Pending.Victim != nil {
			if err := validateBackup(*q.Pending.Victim); err != nil {
				return nil, err
			}
		}
	}
	return &q, nil
}
func validateBackup(entry QueueEntry) error {
	if len(entry.JPEG) == 0 || len(entry.JPEG) > 65520 || entry.CRC == 0 || entry.SHA256 != fmt.Sprintf("%x", sha256.Sum256(entry.JPEG)) || protocol.CRC16IBM(entry.JPEG) != entry.CRC {
		return fmt.Errorf("invalid pending image backup")
	}
	return nil
}
func (q *Queue) Save(path string) error {
	b, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ankerctl-queue-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
