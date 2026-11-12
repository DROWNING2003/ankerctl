package commands

import (
	"context"
	"github.com/DROWNING2003/ankerctl/internal/screen"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
)

func TestFullGalleryBusyUploadRestoresBackup(t *testing.T) {
	q := &screen.Queue{}
	for i := range 8 {
		e := imageEntry([]byte{byte(i + 1), 99})
		e.Slot = uint8(i)
		q.Entries = append(q.Entries, e)
	}
	victim := q.Entries[0]
	other := q.Entries[1]
	data := []byte("new target fixture")
	m := &serial.MockTransport{Responses: [][]byte{
		{10, 28, 0, 8, 1, 255, 0, 60}, currentFixture(1, other.CRC), currentFixture(0, victim.CRC), {10, 26, 0},
		{10, 32, 1}, // new upload rejected busy after deletion
		currentFixture(1, other.CRC), {10, 28, 0, 7, 1, 254, 0, 60}, currentFixture(1, other.CRC),
		{10, 32, 0}, {10, 33, 16}, currentFixture(0, victim.CRC),
	}}
	_, err := runUpload(context.Background(), m, q, func() error { return nil }, data)
	if err == nil {
		t.Fatal("busy failure hidden")
	}
	if q.Pending != nil || len(q.Entries) != 8 {
		t.Fatalf("backup not restored: %+v", q)
	}
	found := false
	for _, e := range q.Entries {
		if e.CRC == victim.CRC {
			found = true
		}
	}
	if !found {
		t.Fatal("oldest lost")
	}
}
