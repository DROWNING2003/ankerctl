package protocol

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupported  = errors.New("unsupported")
	ErrTimeout      = errors.New("communication timeout")
	ErrDisconnected = errors.New("device disconnected")
)

type UnsupportedError struct{ Operation string }

func (e *UnsupportedError) Error() string { return fmt.Sprintf("%s: %s", e.Operation, ErrUnsupported) }
func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }
func Unsupported(operation string) error  { return &UnsupportedError{Operation: operation} }

// CRC16IBM computes the reflected CRC-16/IBM used by the verified screen path.
func CRC16IBM(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
