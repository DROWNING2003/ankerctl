package protocol

import (
	"encoding/binary"
	"testing"
)

// Constructed field-layout fixtures from the analyzed client, not device captures.
func TestPowerSnapshotSeparatesAllChannels(t *testing.T) {
	frame := make([]byte, 16)
	frame[0] = 10
	frame[1] = 0x39
	for _, v := range []struct {
		offset int
		value  uint16
	}{{2, 5000}, {4, 0xff9c}, {6, 6000}, {8, 200}, {10, 20000}, {12, 40000}} {
		binary.BigEndian.PutUint16(frame[v.offset:], v.value)
	}
	frame[14] = 200
	frame[15] = 60
	got, err := ParsePowerSnapshot(frame)
	if err != nil {
		t.Fatal(err)
	}
	if got.UFP.CurrentMa != -100 || got.UFP.PowerMw != -500 {
		t.Fatalf("UFP=%+v", got.UFP)
	}
	if got.PDADC.VoltageMv != 6000 || got.PDADC.CurrentMa != 200 {
		t.Fatalf("PD ADC=%+v", got.PDADC)
	}
	if got.PDIn.VoltageMv != 20000 || got.PDIn.CurrentMa != 40000 || got.PDIn.PowerMw != 800000 {
		t.Fatalf("PD in=%+v", got.PDIn)
	}
	if got.UFPNeg.VoltageMv != 20000 || got.UFPNeg.CurrentMa != 3000 || got.UFPNeg.PowerMw != 60000 {
		t.Fatalf("negotiated=%+v", got.UFPNeg)
	}
}
func TestPortInfoOffsetsAndMinimumSignedCurrent(t *testing.T) {
	frame := make([]byte, 64)
	frame[0] = 10
	frame[1] = 0
	frame[3] = 6
	binary.BigEndian.PutUint16(frame[54:], 5000)
	binary.BigEndian.PutUint16(frame[56:], 0x8000)
	binary.BigEndian.PutUint16(frame[58:], 20000)
	binary.BigEndian.PutUint16(frame[60:], 40000)
	frame[62] = 200
	frame[63] = 60
	got, err := ParsePortInfo(frame)
	if err != nil {
		t.Fatal(err)
	}
	if got.UFP.VoltageMv != 5000 || got.UFP.CurrentMa != -32768 || got.UFP.PowerMw != 163840 {
		t.Fatalf("UFP=%+v", got.UFP)
	}
	if got.PDIn.VoltageMv != 20000 || got.PDIn.CurrentMa != 40000 {
		t.Fatalf("PDIn=%+v", got.PDIn)
	}
	if !got.PDLinked || !got.UFPLinked {
		t.Fatal("missing actual link flags")
	}
}
func TestPowerRejectsIncompleteFrames(t *testing.T) {
	for n := 0; n < 64; n++ {
		f := make([]byte, n)
		if n > 1 {
			f[0] = 10
		}
		if _, err := ParsePortInfo(f); err == nil {
			t.Fatalf("port accepted %d", n)
		}
	}
	for n := 0; n < 16; n++ {
		f := make([]byte, n)
		if n > 1 {
			f[0] = 10
			f[1] = 0x39
		}
		if _, err := ParsePowerSnapshot(f); err == nil {
			t.Fatalf("snapshot accepted %d", n)
		}
	}
}
