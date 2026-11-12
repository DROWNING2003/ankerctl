package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

func TestParseCarouselInterval(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  uint16
	}{{"3", 3}, {"60s", 60}, {"1m", 60}, {"1h", 3600}} {
		n, err := ParseCarouselSeconds(tc.input)
		if err != nil || n != tc.want {
			t.Fatalf("%s: %d %v", tc.input, n, err)
		}
	}
	for _, s := range []string{"", "2", "3601", "1.5s", "-3", "abc", "4294967296"} {
		if _, err := ParseCarouselSeconds(s); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestCarouselMockRequests(t *testing.T) {
	for _, tc := range []struct {
		name, op, value string
		response        []byte
		wantTX          [][]byte
		wantErr         bool
	}{
		{"status", "status", "", []byte{10, 28, 1, 2, 1, 3, 0, 60}, [][]byte{{2, 28}}, false},
		{"on", "on", "", []byte{10, 27, 1}, [][]byte{{2, 31, 1}, {2, 27, 1}}, false},
		{"off", "off", "", []byte{10, 27, 0}, [][]byte{{2, 27, 0}}, false},
		{"on rejected", "on", "", []byte{10, 27, 0}, [][]byte{{2, 31, 1}, {2, 27, 1}}, true},
		{"off rejected", "off", "", []byte{10, 27, 1}, [][]byte{{2, 27, 0}}, true},
		{"interval", "interval", "1m", []byte{10, 29, 0, 60}, [][]byte{{2, 29, 0, 60}}, false},
		{"interval mismatch", "interval", "1m", []byte{10, 29, 0, 30}, [][]byte{{2, 29, 0, 60}}, true},
		{"malformed", "status", "", []byte{10, 28}, [][]byte{{2, 28}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &serial.MockTransport{Responses: [][]byte{tc.response}}
			var out bytes.Buffer
			err := runCarousel(context.Background(), Options{Out: &out, JSON: true, Timeout: time.Second}, tc.op, tc.value, func(Options) (serial.Transport, error) { return m, nil })
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			if !reflect.DeepEqual(m.Sent, tc.wantTX) {
				t.Fatalf("TX=%x want=%x", m.Sent, tc.wantTX)
			}
			if !m.Closed {
				t.Fatal("not closed")
			}
			if err == nil && !json.Valid(out.Bytes()) {
				t.Fatalf("invalid JSON %s", out.String())
			}
			if err != nil && out.Len() != 0 {
				t.Fatalf("failure produced output %s", out.String())
			}
		})
	}
}
func TestCarouselInvalidDoesNotOpen(t *testing.T) {
	open := func(Options) (serial.Transport, error) { t.Fatal("unexpected device access"); return nil, nil }
	o := Options{Timeout: time.Second}
	if err := runCarousel(context.Background(), o, "interval", "2", open); err == nil {
		t.Fatal("accepted invalid interval")
	}
	if err := runCarousel(context.Background(), o, "unknown", "", open); !errors.Is(err, protocol.ErrUnsupported) {
		t.Fatal(err)
	}
	if err := runCarousel(context.Background(), Options{}, "status", "", open); err == nil {
		t.Fatal("accepted zero timeout")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runCarousel(ctx, o, "on", "", open); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCarouselTransportFailure(t *testing.T) {
	m := &serial.MockTransport{Err: context.DeadlineExceeded}
	err := runCarousel(context.Background(), Options{Timeout: time.Second}, "on", "", func(Options) (serial.Transport, error) { return m, nil })
	if !errors.Is(err, context.DeadlineExceeded) || !m.Closed || len(m.Sent) != 1 {
		t.Fatalf("err=%v closed=%t TX=%x", err, m.Closed, m.Sent)
	}
}
func TestCarouselEnableWaitCanCancel(t *testing.T) {
	m := &serial.MockTransport{}
	ctx, cancel := context.WithCancel(context.Background())
	err := runCarousel(ctx, Options{Timeout: time.Second}, "on", "", func(Options) (serial.Transport, error) { cancel(); return m, nil })
	if !errors.Is(err, context.Canceled) || !m.Closed {
		t.Fatalf("%v closed=%t", err, m.Closed)
	}
}
