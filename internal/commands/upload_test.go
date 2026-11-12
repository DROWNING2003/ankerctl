package commands

import (
	"context"
	"errors"
	"github.com/DROWNING2003/ankerctl/internal/screen"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
)

func currentFixture(slot byte, crc uint16) []byte {
	return []byte{10, 24, slot, byte(crc >> 8), byte(crc)}
}
func TestUploadCompleteAndJournal(t *testing.T) {
	data := []byte("test jpeg fixture")
	entry := imageEntry(data)
	q := &screen.Queue{}
	savedPending := false
	m := &serial.MockTransport{Responses: [][]byte{{10, 28, 0, 0, 0, 0, 0, 60}, {10, 32, 0}, {10, 33, 16}, currentFixture(0, entry.CRC)}}
	result, err := runUpload(context.Background(), m, q, func() error {
		if q.Pending != nil {
			savedPending = true
		}
		return nil
	}, data)
	if err != nil || result.Status != "uploaded" || !savedPending || q.Pending != nil || len(q.Entries) != 1 || q.Entries[0].SHA256 != entry.SHA256 {
		t.Fatalf("%+v %v %+v", result, err, q)
	}
}
func TestUploadCRCConflictBeforeWrites(t *testing.T) {
	data := []byte("collision fixture")
	entry := imageEntry(data)
	q := &screen.Queue{Entries: []screen.QueueEntry{{CRC: entry.CRC, Slot: 0, SHA256: "different"}}}
	m := &serial.MockTransport{Responses: [][]byte{{10, 28, 0, 1, 0, 1, 0, 60}}}
	_, err := runUpload(context.Background(), m, q, func() error { return nil }, data)
	if err == nil || len(m.Sent) != 1 {
		t.Fatalf("%v tx=%x", err, m.Sent)
	}
}
func TestDeletedImageCanUploadAgain(t *testing.T) {
	data := []byte("reupload")
	entry := imageEntry(data)
	entry.Slot = 0
	q := &screen.Queue{Entries: []screen.QueueEntry{entry}}
	m := &serial.MockTransport{Responses: [][]byte{{10, 28, 0, 0, 0, 0, 0, 60}, {10, 32, 0}, {10, 33, 16}, currentFixture(0, entry.CRC)}}
	result, err := runUpload(context.Background(), m, q, func() error { return nil }, data)
	if err != nil || result.Status != "uploaded" {
		t.Fatalf("%v %+v", err, result)
	}
}
func TestSaveFailureKeepsRecoveryJournal(t *testing.T) {
	data := []byte("commit failure")
	entry := imageEntry(data)
	q := &screen.Queue{}
	m := &serial.MockTransport{Responses: [][]byte{{10, 28, 0, 0, 0, 0, 0, 60}, {10, 32, 0}, {10, 33, 16}, currentFixture(0, entry.CRC)}}
	calls := 0
	_, err := runUpload(context.Background(), m, q, func() error {
		calls++
		if calls == 2 {
			return errors.New("disk full")
		}
		return nil
	}, data)
	if err == nil || q.Pending == nil {
		t.Fatal("missing recovery journal")
	}
	recovery := &serial.MockTransport{Responses: [][]byte{currentFixture(0, entry.CRC)}}
	if err := recoverUpload(context.Background(), recovery, q, func() error { return nil }); err != nil || q.Pending != nil || len(q.Entries) != 1 {
		t.Fatalf("%v %+v", err, q)
	}
}
func TestRecoverDeletedVictimFromBackup(t *testing.T) {
	victim := imageEntry([]byte("old image"))
	target := imageEntry([]byte("new image"))
	q := &screen.Queue{Entries: []screen.QueueEntry{victim}, Pending: &screen.PendingUpload{Target: target, Victim: &victim, Phase: "prepared"}}
	m := &serial.MockTransport{Responses: [][]byte{currentFixture(1, 123), {10, 28, 0, 7, 1, 254, 0, 60}, currentFixture(1, 123), {10, 32, 0}, {10, 33, 16}, currentFixture(0, victim.CRC)}}
	if err := recoverUpload(context.Background(), m, q, func() error { return nil }); err != nil || q.Pending != nil {
		t.Fatalf("%v %+v", err, q)
	}
}
func TestJournalSaveFailureDoesNotDelete(t *testing.T) {
	data := []byte("new")
	q := &screen.Queue{}
	m := &serial.MockTransport{Responses: [][]byte{{10, 28, 0, 0, 0, 0, 0, 60}}}
	_, err := runUpload(context.Background(), m, q, func() error { return errors.New("disk full") }, data)
	if err == nil || len(m.Sent) != 1 {
		t.Fatalf("%v TX=%x", err, m.Sent)
	}
}
