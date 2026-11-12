package protocol

import (
	"errors"
	"testing"
)

func TestUnsupportedErrorIsStable(t *testing.T) {
	if !errors.Is(Unsupported("status"), ErrUnsupported) {
		t.Fatal("expected errors.Is to identify unsupported operations")
	}
}

func TestCRC16IBM(t *testing.T) {
	if got := CRC16IBM([]byte("123456789")); got != 0xbb3d {
		t.Fatalf("crc=%04x", got)
	}
}
func TestSettingsParsing(t *testing.T) {
	s, err := ParseScreenSettings([]byte{0x0a, 0x16, 50, 2, 1, 1, 0})
	if err != nil || s.Brightness != 50 || !s.ScreensaverOn {
		t.Fatalf("settings=%+v err=%v", s, err)
	}
	if _, err := SetBrightness(9); err == nil {
		t.Fatal("expected brightness range error")
	}
	f, err := ParseFirmwareVersion([]byte{0x0a, 0x30, 'M', 'C', 'U', 0, 0, 0, 0, 0, 'U', 'I', '1', 0, 0, 0, 0, 0})
	if err != nil || f.MCU != "MCU" || f.UI != "UI1" {
		t.Fatalf("firmware=%+v err=%v", f, err)
	}
}
func TestScreenHeaderAndChunkFixture(t *testing.T) {
	h, crc, err := ScreenUploadHeader([]byte{1, 2, 3})
	if err != nil || crc != 0xa110 || h[0] != 2 || h[1] != 32 || h[19] != 0 || h[20] != 55 {
		t.Fatalf("header=%x crc=%x err=%v", h, crc, err)
	}
	p, _ := ScreenDataPacket(1, []byte{9})
	if len(p) != 7 || p[0] != 2 || p[1] != 33 {
		t.Fatalf("packet=%x", p)
	}
}
