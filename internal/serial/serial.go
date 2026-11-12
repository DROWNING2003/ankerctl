package serial

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
)

type Transport interface {
	Request(ctx context.Context, tx []byte) ([]byte, error)
	Send(ctx context.Context, tx []byte) error
	Close() error
}

type Port struct {
	p     serial.Port
	mu    sync.Mutex
	debug func(string, []byte)
}

func Open(path string, timeout time.Duration, debug func(string, []byte)) (*Port, error) {
	if path == "" {
		return nil, fmt.Errorf("serial port path is required")
	}
	p, err := serial.Open(path, &serial.Mode{BaudRate: 115200, DataBits: 8, StopBits: serial.OneStopBit, Parity: serial.NoParity})
	if err == nil && timeout > 0 {
		_ = p.SetReadTimeout(timeout)
	}
	if err != nil {
		return nil, err
	}
	return &Port{p: p, debug: debug}, nil
}
func (p *Port) writeLocked(ctx context.Context, tx []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.debug != nil {
		p.debug("TX", tx)
	}
	for written := 0; written < len(tx); {
		n, err := p.p.Write(tx[written:])
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("serial write made no progress")
		}
		written += n
	}
	return nil
}
func (p *Port) Send(ctx context.Context, tx []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writeLocked(ctx, tx)
}
func (p *Port) Request(ctx context.Context, tx []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.writeLocked(ctx, tx); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	out := make([]byte, 0, 256)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		n, err := p.p.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("device disconnected: %w", err)
			}
			return nil, err
		}
		if n > 0 {
			out = append(out, buf[:n]...)
			if p.debug != nil {
				p.debug("RX", buf[:n])
			}
			return out, nil
		}
	}
}
func (p *Port) Close() error { return p.p.Close() }
