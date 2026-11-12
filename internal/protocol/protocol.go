package protocol

import (
	"errors"
	"fmt"
	"time"
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

// Parser is deliberately conservative. The A210C frame grammar is not yet
// authenticated; this only supports an explicitly supplied fixture header.
type Parser struct{ buf, prefix []byte }

func NewParser(prefix []byte) *Parser { return &Parser{prefix: append([]byte(nil), prefix...)} }
func (p *Parser) Buffered() int       { return len(p.buf) }
func (p *Parser) Push(data []byte) ([][]byte, error) {
	p.buf = append(p.buf, data...)
	var out [][]byte
	for {
		i := indexPrefix(p.buf, p.prefix)
		if i < 0 {
			if len(p.buf) > len(p.prefix) {
				p.buf = p.buf[len(p.buf)-len(p.prefix):]
			}
			return out, nil
		}
		if i > 0 {
			p.buf = p.buf[i:]
		}
		if len(p.buf) < len(p.prefix)+1 {
			return out, nil
		}
		length := int(p.buf[len(p.prefix)])
		if length == 0 {
			length = len(p.prefix) + 1
		}
		if len(p.buf) < length {
			return out, nil
		}
		out = append(out, append([]byte(nil), p.buf[:length]...))
		p.buf = p.buf[length:]
	}
}
func indexPrefix(buf, prefix []byte) int {
	if len(prefix) == 0 {
		return 0
	}
	for i := 0; i+len(prefix) <= len(buf); i++ {
		match := true
		for j := range prefix {
			if buf[i+j] != prefix[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func WaitResponse(read func() ([]byte, error), timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ch := make(chan struct {
		b   []byte
		err error
	}, 1)
	go func() {
		b, err := read()
		ch <- struct {
			b   []byte
			err error
		}{b, err}
	}()
	select {
	case r := <-ch:
		return r.b, r.err
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}
