package protocol

import (
	"bytes"
	"testing"
)

// These are constructed fixtures from the shipped client's field definitions,
// not captured A210C responses.
func TestTimerDurationRequest(t *testing.T) {
	got, err := TimerDurationRequest(1500)
	if err != nil || !bytes.Equal(got, []byte{2, 0x2a, 0xdc, 5, 0, 0}) {
		t.Fatalf("got %x, %v", got, err)
	}
	for _, seconds := range []uint32{0, 29, 31, 3601} {
		if _, err := TimerDurationRequest(seconds); err == nil {
			t.Fatalf("accepted %d", seconds)
		}
	}
}
func TestTimerResponses(t *testing.T) {
	duration, err := ParseTimerDuration([]byte{10, 0x2a, 0xdc, 5, 0, 0})
	if err != nil || duration != 1500 {
		t.Fatalf("%d %v", duration, err)
	}
	status, err := ParseTimerStatus([]byte{10, 0x2b, 1, 60, 0, 0, 0, 2, 0, 0, 0})
	if err != nil || status.RemainingSeconds != 60 || status.CompletionSeq != 2 || status.State != 1 {
		t.Fatalf("%+v %v", status, err)
	}
	stats, err := ParseTimerStats([]byte{10, 0x2d, 2, 0, 0, 0, 0xb8, 0xb, 0, 0})
	if err != nil || stats.CompletedCount != 2 || stats.CompletedSeconds != 3000 {
		t.Fatalf("%+v %v", stats, err)
	}
	for _, f := range [][]byte{nil, {10, 0x2b, 1}, {10, 0x2a, 1, 0, 0, 0, 0, 0, 0, 0, 0}} {
		if _, err := ParseTimerStatus(f); err == nil {
			t.Fatalf("accepted malformed %x", f)
		}
	}
	if err := ParseTimerStart([]byte{10, 0x2c, 0}); err == nil {
		t.Fatal("accepted rejected start")
	}
	if err := ParseTimerStart([]byte{10, 0x2c, 1}); err != nil {
		t.Fatal(err)
	}
}
