package serial

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	driver "go.bug.st/serial"
)

type Transport interface {
	Request(context.Context, []byte) ([]byte, error)
	Send(context.Context, []byte) error
	Close() error
}

// lowPort permits testing without touching hardware. Close must unblock Read/Write.
type lowPort interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	SetReadTimeout(time.Duration) error
	Close() error
}
type Port struct {
	p         lowPort
	gate      chan struct{}
	done      chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
	timeout   time.Duration
	pending   []byte // only accessed by the gate owner
	debug     func(string, []byte)
}

// CheckAvailable is a read-only best-effort owner check. It never stops processes.
// The driver requests TIOCEXCL on Unix but v1.6.0 ignores its failure, so do not
// rely on that alone. lsof is required; a race between this check and open remains.
func CheckAvailable(ctx context.Context, path string) error {
	tool, err := exec.LookPath("lsof")
	if err != nil {
		return fmt.Errorf("cannot verify serial ownership: lsof is required: %w", err)
	}
	args := []string{"-t", "--", path}
	// macOS callout and dial-in nodes address the same device.
	if strings.HasPrefix(path, "/dev/cu.") {
		args = append(args, "/dev/tty."+strings.TrimPrefix(path, "/dev/cu."))
	}
	if strings.HasPrefix(path, "/dev/tty.") {
		args = append(args, "/dev/cu."+strings.TrimPrefix(path, "/dev/tty."))
	}
	out, err := exec.CommandContext(ctx, tool, args...).CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return fmt.Errorf("serial ownership check found an owner or diagnostic; refusing to open %s", path)
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("cannot verify serial ownership: %w", err)
	}
	return nil
}
func Open(path string, timeout time.Duration, debug func(string, []byte)) (*Port, error) {
	if path == "" {
		return nil, fmt.Errorf("serial port path is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("serial timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := CheckAvailable(ctx, path); err != nil {
		return nil, err
	}
	p, err := driver.Open(path, &driver.Mode{BaudRate: 115200, DataBits: 8, StopBits: driver.OneStopBit, Parity: driver.NoParity})
	if err != nil {
		return nil, err
	}
	return newPort(p, timeout, debug)
}
func newPort(p lowPort, timeout time.Duration, debug func(string, []byte)) (*Port, error) {
	if timeout <= 0 {
		_ = p.Close()
		return nil, fmt.Errorf("serial timeout must be positive")
	}
	// Fixed short polling bounds read cancellation without changing timeout mid-read.
	if err := p.SetReadTimeout(min(timeout, 25*time.Millisecond)); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("set serial read timeout: %w", err)
	}
	return &Port{p: p, gate: make(chan struct{}, 1), done: make(chan struct{}), timeout: timeout, debug: debug}, nil
}
func (p *Port) Close() error {
	p.closeOnce.Do(func() { p.closed.Store(true); close(p.done); p.closeErr = p.p.Close() })
	return p.closeErr
}
func (p *Port) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return fmt.Errorf("serial port closed")
	case p.gate <- struct{}{}:
	}
	if err := ctx.Err(); err != nil {
		<-p.gate
		return err
	}
	if p.closed.Load() {
		<-p.gate
		return fmt.Errorf("serial port closed")
	}
	return nil
}
func (p *Port) operation(ctx context.Context, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if err := p.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-p.gate }()
	// No abandoned reader goroutines: cancellation closes the owned port and all
	// I/O stays synchronous. A canceled in-flight operation poisons this connection.
	stop := context.AfterFunc(ctx, func() { _ = p.Close() })
	defer stop()
	err := fn(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func (p *Port) write(ctx context.Context, b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if p.debug != nil {
		p.debug("TX", b)
	}
	for offset := 0; offset < len(b); {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.closed.Load() {
			return fmt.Errorf("serial port closed")
		}
		n, err := p.p.Write(b[offset:])
		if n < 0 || n > len(b)-offset {
			return fmt.Errorf("invalid serial write count %d", n)
		}
		offset += n
		if err != nil {
			return fmt.Errorf("serial write: %w", err)
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}
func (p *Port) Send(ctx context.Context, b []byte) error {
	return p.operation(ctx, func(ctx context.Context) error { return p.write(ctx, b) })
}

// Lengths are from Dock Manager 1.1.7 tryConsumeOneCdcFrame, formatted lines
// 98656..98789. No generic guessed length field or wire CRC is assumed.
func responseLength(command byte) (int, bool) {
	n, ok := map[byte]int{0: 64, 2: 42, 3: 22, 22: 7, 24: 5, 26: 3, 27: 3, 28: 8, 29: 4, 32: 3, 33: 13, 39: 9, 41: 4, 42: 6, 43: 11, 44: 3, 45: 10, 46: 6, 48: 18, 54: 6, 56: 3, 57: 16, 59: 3, 60: 3, 61: 3}[command]
	return n, ok
}
func expectedResponse(tx []byte) (byte, error) {
	if len(tx) == 0 {
		return 33, nil
	}
	if len(tx) < 2 || tx[0] != 2 {
		return 0, fmt.Errorf("unsupported serial request format")
	}
	command := tx[1]
	if command == 10 {
		command = 2
	}
	switch tx[1] {
	case 0, 10, 11, 22, 24, 26, 27, 28, 29, 32, 39, 42, 43, 44, 45, 46, 48, 54, 56, 57:
		if tx[1] == 11 {
			command = 3
		}
		return command, nil
	default:
		return 0, fmt.Errorf("unsupported serial request command 0x%02x", tx[1])
	}
}
func (p *Port) nextFrame() ([]byte, error) {
	for len(p.pending) > 0 {
		if p.pending[0] != 10 {
			return nil, fmt.Errorf("unexpected serial frame prefix 0x%02x", p.pending[0])
		}
		if len(p.pending) < 2 {
			return nil, nil
		}
		n, ok := responseLength(p.pending[1])
		if !ok {
			return nil, fmt.Errorf("unsupported asynchronous response 0A%02X", p.pending[1])
		}
		if len(p.pending) < n {
			return nil, nil
		}
		frame := append([]byte(nil), p.pending[:n]...)
		p.pending = p.pending[n:]
		return frame, nil
	}
	return nil, nil
}
func (p *Port) Request(ctx context.Context, tx []byte) (result []byte, err error) {
	want, err := expectedResponse(tx)
	if err != nil {
		return nil, err
	}
	err = p.operation(ctx, func(ctx context.Context) error {
		if err := p.write(ctx, tx); err != nil {
			return err
		}
		buf := make([]byte, 4096)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			frame, err := p.nextFrame()
			if err != nil {
				_ = p.Close() // Unknown framing cannot be resynchronized safely.
				return err
			}
			if frame != nil {
				if frame[1] == want && (want != 33 || frame[2] == 16 || frame[2] == 17) {
					result = frame
					return nil
				}
				continue
			}
			n, err := p.p.Read(buf)
			if n < 0 || n > len(buf) {
				return fmt.Errorf("invalid serial read count %d", n)
			}
			if n > 0 {
				if p.debug != nil {
					p.debug("RX", buf[:n])
				}
				p.pending = append(p.pending, buf[:n]...)
			}
			if err != nil {
				return fmt.Errorf("serial read/disconnect: %w", err)
			}
			if n == 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-p.done:
					return fmt.Errorf("serial port closed")
				case <-time.After(time.Millisecond):
				}
			}
		}
	})
	return result, err
}
