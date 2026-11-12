package commands

import (
	"bytes"
	"context"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
	"time"
)

func TestControlReadbacks(t *testing.T) {
	for _, tc := range []struct {
		op, value     string
		tx, query, rx []byte
	}{
		{"timeout", "5m", []byte{2, 17, 2}, []byte{2, 22}, []byte{10, 22, 30, 2, 1, 1, 0}},
		{"language", "de", []byte{2, 16, 3}, []byte{2, 22}, []byte{10, 22, 30, 1, 3, 1, 0}},
		{"screen-on", "", []byte{2, 31, 1}, []byte{2, 22}, []byte{10, 22, 30, 1, 1, 1, 0}},
		{"screen-off", "", []byte{2, 31, 0}, []byte{2, 22}, []byte{10, 22, 30, 1, 1, 0, 0}},
		{"select", "1253", []byte{2, 25, 0x12, 0x53}, []byte{2, 24}, []byte{10, 24, 3, 0x12, 0x53}},
	} {
		t.Run(tc.op, func(t *testing.T) {
			m := &serial.MockTransport{Responses: [][]byte{tc.rx}}
			var out bytes.Buffer
			err := runControl(context.Background(), Options{Timeout: time.Second, Out: &out}, tc.op, tc.value, func(Options) (serial.Transport, error) { return m, nil })
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Sent) != 2 || !bytes.Equal(m.Sent[0], tc.tx) || !bytes.Equal(m.Sent[1], tc.query) {
				t.Fatalf("TX=%x", m.Sent)
			}
		})
	}
}
func TestControlMismatchAndDeletionFailure(t *testing.T) {
	for _, tc := range []struct {
		op, value string
		rx        []byte
	}{
		{"timeout", "5m", []byte{10, 22, 30, 1, 1, 1, 0}},
		{"screen-off", "", []byte{10, 22, 30, 1, 1, 1, 0}},
		{"delete", "1234", []byte{10, 26, 1}},
		{"delete", "1234", []byte{10, 26, 2}},
		{"reminder", "30m", []byte{10, 46, 60, 0, 0, 0}},
		{"focus-on", "", append([]byte{10, 2}, make([]byte, 40)...)},
	} {
		m := &serial.MockTransport{Responses: [][]byte{tc.rx}}
		err := runControl(context.Background(), Options{Timeout: time.Second}, tc.op, tc.value, func(Options) (serial.Transport, error) { return m, nil })
		if err == nil || !m.Closed {
			t.Fatalf("op=%s err=%v", tc.op, err)
		}
	}
}
