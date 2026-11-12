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

// Constructed fixtures from client offsets, not captured device frames.
func detailFixture() []byte {
	f := make([]byte, 64)
	f[0] = 10
	f[3] = 7
	f[8] = 1
	f[9] = 6
	f[12] = 128
	binary.BigEndian.PutUint16(f[4:], 0x1234)
	binary.BigEndian.PutUint16(f[6:], 0x5678)
	binary.BigEndian.PutUint16(f[13:], 2560)
	binary.BigEndian.PutUint16(f[15:], 1440)
	f[17] = 60
	binary.BigEndian.PutUint16(f[18:], 1920)
	binary.BigEndian.PutUint16(f[20:], 1080)
	f[22] = 255
	f[23] = 3
	copy(f[24:], []byte{'D', 'E', 'L', 0x12, 0x34})
	copy(f[29:], []byte{'A', 'C', 'R', 0xab, 0xcd})
	copy(f[34:], "DELL TEST")
	return f
}
func TestDetailedPortsFields(t *testing.T) {
	got, err := ParseDetailedPortInfo(detailFixture())
	if err != nil {
		t.Fatal(err)
	}
	if !got.EthernetLinked || !got.PDLinked || !got.UFPLinked || got.HostVendorIndex != 0x1234 || got.HostProductIndex != 0x5678 {
		t.Fatalf("%+v", got)
	}
	if got.USB.C1.Speed != "10 Gbps" || got.USB.C1.RawFlags != 6 || got.USB.A3.Speed != "" || !got.USB.A3.Connected {
		t.Fatalf("%+v", got.USB)
	}
	if got.DP.Width != 2560 || got.DP.RefreshHz == nil || *got.DP.RefreshHz != 60 || got.DP.Name != "DELL TEST" || got.DP.ProductCode != "1234" || got.DP.Manufacturer != "DEL" {
		t.Fatalf("%+v", got.DP)
	}
	if got.HDMI.RefreshHz != nil {
		t.Fatal("invented refresh from invalid value")
	}
}
func TestDisplaysRejectInvalidResolutionAndMetadata(t *testing.T) {
	f := detailFixture()
	binary.BigEndian.PutUint16(f[13:], 8000)
	f[29] = 0
	got, err := ParseDetailedPortInfo(f)
	if err != nil {
		t.Fatal(err)
	}
	if got.DP.Connected || got.DP.Width != 0 {
		t.Fatal("invalid resolution accepted")
	}
	if got.HDMI.Manufacturer != "" {
		t.Fatal("invalid EDID manufacturer mapped")
	}
}
func TestPhysicalSizes(t *testing.T) {
	f := []byte{10, 39, 3, 1, 14, 69, 0, 240, 61}
	got, err := ParsePhysicalSizes(f)
	if err != nil {
		t.Fatal(err)
	}
	if got.DP.DiagonalInches == nil || *got.DP.DiagonalInches != 27 || got.HDMI.DiagonalInches == nil || *got.HDMI.DiagonalInches != 24 {
		t.Fatalf("%+v", got)
	}
	f[2] = 0
	got, _ = ParsePhysicalSizes(f)
	if got.DP.DiagonalInches != nil || got.DP.RawCentimeters != 0 {
		t.Fatal("ignored validity mask")
	}
}
func TestDetailedMalformedFrames(t *testing.T) {
	for _, f := range [][]byte{nil, {10, 0}, make([]byte, 64)} {
		if _, err := ParseDetailedPortInfo(f); err == nil {
			t.Fatal("accepted invalid port frame")
		}
	}
	for _, f := range [][]byte{nil, {10, 39}, make([]byte, 9)} {
		if _, err := ParsePhysicalSizes(f); err == nil {
			t.Fatal("accepted invalid physical frame")
		}
	}
}
