package protocol

import (
	"encoding/binary"
	"fmt"
)

// Timer field layouts follow Dock Manager 1.1.7's real CDC methods, not its mocks.
type TimerStatus struct {
	State            uint8  `json:"state"`
	RemainingSeconds uint32 `json:"remaining_seconds"`
	CompletionSeq    uint32 `json:"completion_seq"`
}
type TimerStats struct {
	CompletedCount   uint32 `json:"completed_count"`
	CompletedSeconds uint32 `json:"completed_seconds"`
}

func TimerDurationRequest(seconds uint32) ([]byte, error) {
	if seconds < 30 || seconds > 3600 || seconds%30 != 0 {
		return nil, fmt.Errorf("duration must be 30..3600 seconds and a multiple of 30")
	}
	frame := []byte{2, 0x2a, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(frame[2:], seconds)
	return frame, nil
}
func checkTimerFrame(frame []byte, command byte, length int) error {
	if len(frame) != length || frame[0] != 10 || frame[1] != command {
		return fmt.Errorf("invalid timer response: expected 0A%02X (%d bytes), got %d bytes", command, length, len(frame))
	}
	return nil
}
func ParseTimerDuration(frame []byte) (uint32, error) {
	if err := checkTimerFrame(frame, 0x2a, 6); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(frame[2:]), nil
}
func ParseTimerStatus(frame []byte) (TimerStatus, error) {
	if err := checkTimerFrame(frame, 0x2b, 11); err != nil {
		return TimerStatus{}, err
	}
	return TimerStatus{frame[2], binary.LittleEndian.Uint32(frame[3:]), binary.LittleEndian.Uint32(frame[7:])}, nil
}
func ParseTimerStats(frame []byte) (TimerStats, error) {
	if err := checkTimerFrame(frame, 0x2d, 10); err != nil {
		return TimerStats{}, err
	}
	return TimerStats{binary.LittleEndian.Uint32(frame[2:]), binary.LittleEndian.Uint32(frame[6:])}, nil
}
func ParseTimerStart(frame []byte) error {
	if err := checkTimerFrame(frame, 0x2c, 3); err != nil {
		return err
	}
	if frame[2] == 0 {
		return fmt.Errorf("device rejected timer start")
	}
	return nil
}
