package protocol

import (
	"encoding/binary"
	"fmt"
)

type ScreenSettings struct {
	Brightness, LockTime, Language uint8
	ScreensaverOn                  bool
}

func ParseScreenSettings(frame []byte) (ScreenSettings, error) {
	if len(frame) < 7 || frame[0] != 0x0a || frame[1] != 0x16 {
		return ScreenSettings{}, fmt.Errorf("invalid 0A16 response")
	}
	return ScreenSettings{frame[2], frame[3], frame[4], frame[5] == 1}, nil
}
func ScreenSettingsRequest() []byte { return []byte{2, 22} }
func SetBrightness(value int) ([]byte, error) {
	if value < 10 || value > 100 {
		return nil, fmt.Errorf("brightness must be between 10 and 100")
	}
	return []byte{2, 18, byte(value)}, nil
}
func SetLockTime(enum uint8) ([]byte, error) {
	if enum > 5 {
		return nil, fmt.Errorf("lock time enum must be 0..5")
	}
	return []byte{2, 17, enum}, nil
}

type FirmwareVersion struct{ MCU, UI string }

func ParseFirmwareVersion(frame []byte) (FirmwareVersion, error) {
	if len(frame) < 18 || frame[0] != 0x0a || frame[1] != 0x30 {
		return FirmwareVersion{}, fmt.Errorf("invalid 0A30 response")
	}
	return FirmwareVersion{ascii(frame[2:10]), ascii(frame[10:18])}, nil
}
func ascii(b []byte) string {
	for i, c := range b {
		if c == 0 {
			b = b[:i]
			break
		}
	}
	return string(b)
}

type CurrentScreen struct {
	Slot uint8
	CRC  uint16
}

func ParseCurrentScreen(frame []byte) (*CurrentScreen, error) {
	if len(frame) < 5 || frame[0] != 0x0a || frame[1] != 0x18 {
		return nil, fmt.Errorf("invalid 0A18 response")
	}
	if binary.BigEndian.Uint16(frame[3:5]) == 0 {
		return nil, nil
	}
	return &CurrentScreen{frame[2], binary.BigEndian.Uint16(frame[3:5])}, nil
}
