package protocol

import (
	"bytes"
	"testing"
)

// All frames here are constructed from the analyzed client, not hardware captures.
func TestCarouselRequests(t *testing.T) {
	if !bytes.Equal(CarouselStatusRequest(), []byte{2, 0x1c}) || !bytes.Equal(CarouselEnableScreensaverRequest(), []byte{2, 0x1f, 1}) {
		t.Fatal("wrong query/prerequisite frames")
	}
	for _, enabled := range []bool{false, true} {
		want := byte(0)
		if enabled {
			want = 1
		}
		if !bytes.Equal(CarouselModeRequest(enabled), []byte{2, 0x1b, want}) {
			t.Fatal("wrong mode frame")
		}
	}
	for _, seconds := range []uint16{3, 60, 3600} {
		got, err := CarouselIntervalRequest(seconds)
		if err != nil || !bytes.Equal(got, []byte{2, 0x1d, byte(seconds >> 8), byte(seconds)}) {
			t.Fatalf("%x %v", got, err)
		}
	}
	for _, seconds := range []uint16{0, 2, 3601, 65535} {
		if _, err := CarouselIntervalRequest(seconds); err == nil {
			t.Fatalf("accepted %d", seconds)
		}
	}
}
func TestCarouselResponses(t *testing.T) {
	state, err := ParseCarouselStatus([]byte{10, 0x1c, 1, 3, 2, 7, 0x0e, 0x10})
	if err != nil || !state.Enabled || state.Count != 3 || state.Slot != 2 || state.Bitmap != 7 || state.IntervalSeconds != 3600 {
		t.Fatalf("%+v %v", state, err)
	}
	enabled, err := ParseCarouselMode([]byte{10, 0x1b, 0})
	if err != nil || enabled {
		t.Fatalf("%v %v", enabled, err)
	}
	seconds, err := ParseCarouselInterval([]byte{10, 0x1d, 0, 60})
	if err != nil || seconds != 60 {
		t.Fatalf("%d %v", seconds, err)
	}
}
func TestCarouselRejectsMalformedResponses(t *testing.T) {
	for _, frame := range [][]byte{nil, {10, 0x1c}, {9, 0x1c, 1, 0, 0, 0, 0, 60}, {10, 0x1b, 1, 0, 0, 0, 0, 60}, {10, 0x1c, 2, 0, 0, 0, 0, 60}, {10, 0x1c, 1, 0, 0, 0, 0, 60, 0}} {
		if _, err := ParseCarouselStatus(frame); err == nil {
			t.Fatalf("accepted status %x", frame)
		}
	}
	for _, frame := range [][]byte{nil, {10, 0x1b}, {9, 0x1b, 1}, {10, 0x1c, 1}, {10, 0x1b, 2}, {10, 0x1b, 1, 0}} {
		if _, err := ParseCarouselMode(frame); err == nil {
			t.Fatalf("accepted mode %x", frame)
		}
	}
	for _, frame := range [][]byte{nil, {10, 0x1d, 0}, {9, 0x1d, 0, 60}, {10, 0x1c, 0, 60}, {10, 0x1d, 0, 60, 0}, {10, 0x1d, 0, 2}, {10, 0x1d, 0x0e, 0x11}} {
		if _, err := ParseCarouselInterval(frame); err == nil {
			t.Fatalf("accepted interval %x", frame)
		}
	}
}
