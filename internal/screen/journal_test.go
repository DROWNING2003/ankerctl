package screen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"path/filepath"
	"testing"
)

func TestQueueStrongIdentity(t *testing.T) {
	q := Queue{Entries: []QueueEntry{{CRC: 11, Slot: 0, SHA256: "aaa"}}}
	if err := q.CheckIdentity(11, "bbb"); err == nil {
		t.Fatal("CRC collision accepted")
	}
	if err := q.CheckIdentity(11, "aaa"); err != nil {
		t.Fatal(err)
	}
	q.Entries[0].SHA256 = ""
	if err := q.CheckIdentity(11, "aaa"); err == nil {
		t.Fatal("legacy identity accepted")
	}
}
func TestQueueJournalRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := Queue{Pending: &PendingUpload{Target: QueueEntry{CRC: protocol.CRC16IBM([]byte{1, 2}), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte{1, 2})), JPEG: []byte{1, 2}}, Phase: "prepared"}}
	if err := q.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadQueue(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Pending == nil || !bytes.Equal(loaded.Pending.Target.JPEG, []byte{1, 2}) {
		t.Fatal("journal lost")
	}
}
func TestQueueRemove(t *testing.T) {
	q := Queue{Entries: []QueueEntry{{CRC: 11, Slot: 0}, {CRC: 22, Slot: 1}}}
	q.Remove(11)
	if len(q.Entries) != 1 || q.Entries[0].CRC != 22 {
		t.Fatal(q)
	}
}
