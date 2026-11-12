package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

func TestTimerDurationInputs(t *testing.T) {
	for _, s := range []string{"1500", "25m", "90s", "1h"} {
		if _, err := ParseTimerSeconds(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"0", "-30", "29s", "31", "1.5s", "2h", "garbage", "4294967296"} {
		if _, err := ParseTimerSeconds(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestTimerMockResponses(t *testing.T) {
	for _, tc := range []struct {
		op, value string
		frame     []byte
		wantErr   bool
	}{
		{"status", "", []byte{10, 43, 1, 60, 0, 0, 0, 2, 0, 0, 0}, false},
		{"duration", "25m", []byte{10, 42, 0xdc, 5, 0, 0}, false},
		{"duration", "", []byte{10, 42, 0xdc, 5, 0, 0}, false},
		{"duration", "25m", []byte{10, 42, 60, 0, 0, 0}, true},
		{"start", "", []byte{10, 44, 0}, true},
		{"start", "", []byte{10, 44, 1}, false},
		{"stats", "", []byte{10, 45, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{"status", "", []byte{10, 43}, true},
	} {
		t.Run(tc.op+tc.value+string(tc.frame), func(t *testing.T) {
			m := &serial.MockTransport{Responses: [][]byte{tc.frame}}
			var out bytes.Buffer
			err := runTimer(context.Background(), Options{JSON: true, Out: &out, Timeout: time.Second}, tc.op, tc.value, func(Options) (serial.Transport, error) { return m, nil })
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			expected := map[string][]byte{"status": {2, 43}, "duration": {2, 42, 0xdc, 5, 0, 0}, "start": {2, 44}, "stats": {2, 45}}[tc.op]
			if tc.op == "duration" && tc.value == "" {
				expected = []byte{2, 42}
			}
			if len(m.Sent) != 1 || !bytes.Equal(m.Sent[0], expected) {
				t.Fatalf("TX=%x want=%x", m.Sent, expected)
			}
			if !m.Closed {
				t.Fatal("transport not closed")
			}
			if err == nil && !json.Valid(out.Bytes()) {
				t.Fatalf("invalid JSON %s", out.String())
			}
		})
	}
}
func TestTimerUnsupportedAndInvalidDoNotOpen(t *testing.T) {
	open := func(Options) (serial.Transport, error) { t.Fatal("opened device unexpectedly"); return nil, nil }
	err := runTimer(context.Background(), Options{}, "stop", "", open)
	if !errors.Is(err, protocol.ErrUnsupported) {
		t.Fatal(err)
	}
	if err := runTimer(context.Background(), Options{}, "duration", "31", open); err == nil {
		t.Fatal("accepted invalid input")
	}
}
func TestTimerTransportError(t *testing.T) {
	m := &serial.MockTransport{Err: context.DeadlineExceeded}
	err := runTimer(context.Background(), Options{Timeout: time.Second}, "status", "", func(Options) (serial.Transport, error) { return m, nil })
	if !errors.Is(err, context.DeadlineExceeded) || !m.Closed {
		t.Fatalf("%v closed=%t", err, m.Closed)
	}
}
