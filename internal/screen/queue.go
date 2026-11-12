package screen

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
)

type QueueEntry struct {
	CRC  uint16 `json:"crc16"`
	Slot uint8  `json:"slot"`
}
type Queue struct {
	Entries []QueueEntry `json:"entries"`
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
	return &q, nil
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
	return os.Rename(name, path)
}
