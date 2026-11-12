package commands

import (
	"context"
	"testing"

	"github.com/DROWNING2003/ankerctl/internal/screen"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

func TestEvictVerifiesOldestBeforeDeleting(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 24, 1, 0, 22}, {10, 24, 0, 0, 11}, {10, 26, 0}}}
	if err := evictOldest(context.Background(), m, screen.QueueEntry{CRC: 11, Slot: 0}); err != nil {
		t.Fatal(err)
	}
	last := m.Sent[len(m.Sent)-1]
	if len(last) != 4 || last[0] != 2 || last[1] != 26 || last[3] != 11 {
		t.Fatalf("wrong delete %x", last)
	}
}
func TestEvictRefusesMismatchedCRC(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 24, 1, 0, 22}, {10, 24, 0, 0, 33}}}
	if err := evictOldest(context.Background(), m, screen.QueueEntry{CRC: 11, Slot: 0}); err == nil {
		t.Fatal("accepted mismatch")
	}
	for _, f := range m.Sent {
		if len(f) > 1 && f[1] == 26 {
			t.Fatal("sent deletion before identity check")
		}
	}
}
