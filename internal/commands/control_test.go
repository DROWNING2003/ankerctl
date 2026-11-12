package commands

import (
	"bytes"
	"context"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
	"time"
)

func TestControlFrames(t *testing.T) {
	for _, tc := range []struct {
		op, value string
		tx        []byte
		rx        []byte
	}{
		{"reminder", "30m", []byte{2, 46, 8, 7, 0, 0}, []byte{10, 46, 8, 7, 0, 0}},
		{"reminder", "", []byte{2, 46}, []byte{10, 46, 8, 7, 0, 0}},
		{"focus-status", "", []byte{2, 10}, append([]byte{10, 2}, make([]byte, 40)...)},
		{"focus-on", "", []byte{2, 10, 1}, append([]byte{10, 2, 1}, make([]byte, 39)...)},
		{"focus-off", "", []byte{2, 10, 0}, append([]byte{10, 2}, make([]byte, 40)...)},
		{"delete", "a7fa", []byte{2, 26, 0xa7, 0xfa}, []byte{10, 26, 0}},
	} {
		t.Run(tc.op+tc.value, func(t *testing.T) {
			m := &serial.MockTransport{Responses: [][]byte{tc.rx}}
			var out bytes.Buffer
			err := runControl(context.Background(), Options{Timeout: time.Second, Out: &out, JSON: true}, tc.op, tc.value, func(Options) (serial.Transport, error) { return m, nil })
			if err != nil {
				t.Fatal(err)
			}
			if !m.Closed || len(m.Sent) != 1 || !bytes.Equal(m.Sent[0], tc.tx) {
				t.Fatalf("sent=%x closed=%v", m.Sent, m.Closed)
			}
		})
	}
}
func TestControlInvalidBeforeOpen(t *testing.T) {
	for _, tc := range [][2]string{{"timeout", "90s"}, {"language", "xx"}, {"reminder", "31"}, {"select", "garbage"}, {"delete", "0"}} {
		err := runControl(context.Background(), Options{Timeout: time.Second}, tc[0], tc[1], func(Options) (serial.Transport, error) { t.Fatal("unexpected open"); return nil, nil })
		if err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}
