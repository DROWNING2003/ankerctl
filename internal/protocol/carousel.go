package protocol

import (
	"encoding/binary"
	"fmt"
)

// CarouselState describes the firmware's stored image carousel, not an image list.
type CarouselState struct {
	Enabled         bool   `json:"enabled"`
	Count           uint8  `json:"count"`
	Slot            uint8  `json:"slot"`
	Bitmap          uint8  `json:"bitmap"`
	IntervalSeconds uint16 `json:"interval_seconds"`
}

func CarouselStatusRequest() []byte { return []byte{2, 0x1c} }

// Enabling the carousel requires the screensaver master switch first.
func CarouselEnableScreensaverRequest() []byte { return []byte{2, 0x1f, 1} }
func CarouselModeRequest(enabled bool) []byte {
	value := byte(0)
	if enabled {
		value = 1
	}
	return []byte{2, 0x1b, value}
}
func CarouselIntervalRequest(seconds uint16) ([]byte, error) {
	if seconds < 3 || seconds > 3600 {
		return nil, fmt.Errorf("carousel interval must be 3..3600 whole seconds")
	}
	frame := []byte{2, 0x1d, 0, 0}
	binary.BigEndian.PutUint16(frame[2:], seconds)
	return frame, nil
}
func checkCarouselFrame(frame []byte, command byte, length int) error {
	if len(frame) != length || frame[0] != 10 || frame[1] != command {
		return fmt.Errorf("invalid carousel response: expected 0A%02X (%d bytes), got %d bytes", command, length, len(frame))
	}
	return nil
}
func ParseCarouselStatus(frame []byte) (CarouselState, error) {
	if err := checkCarouselFrame(frame, 0x1c, 8); err != nil {
		return CarouselState{}, err
	}
	if frame[2] > 1 {
		return CarouselState{}, fmt.Errorf("invalid carousel enabled value %d", frame[2])
	}
	return CarouselState{frame[2] == 1, frame[3], frame[4], frame[5], binary.BigEndian.Uint16(frame[6:])}, nil
}
func ParseCarouselMode(frame []byte) (bool, error) {
	if err := checkCarouselFrame(frame, 0x1b, 3); err != nil {
		return false, err
	}
	if frame[2] > 1 {
		return false, fmt.Errorf("invalid carousel enabled value %d", frame[2])
	}
	return frame[2] == 1, nil
}
func ParseCarouselInterval(frame []byte) (uint16, error) {
	if err := checkCarouselFrame(frame, 0x1d, 4); err != nil {
		return 0, err
	}
	seconds := binary.BigEndian.Uint16(frame[2:])
	if seconds < 3 || seconds > 3600 {
		return 0, fmt.Errorf("invalid carousel interval echo %d", seconds)
	}
	return seconds, nil
}
