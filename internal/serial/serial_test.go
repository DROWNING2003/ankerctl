package serial

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Synthetic streams follow fixed lengths observed in Dock Manager, not hardware captures.
type fakePort struct {
	mu         sync.Mutex
	chunks     [][]byte
	writes     []byte
	maxWrite   int
	timeoutErr error
	readErr    error
	blocked    bool
	blockWrite bool
	writeErr   error
	zeroWrite  bool
	entered    chan struct{}
	done       chan struct{}
	once       sync.Once
}

func fake(chunks ...[]byte) *fakePort                  { return &fakePort{chunks: chunks, done: make(chan struct{})} }
func (f *fakePort) SetReadTimeout(time.Duration) error { return f.timeoutErr }
func (f *fakePort) Close() error                       { f.once.Do(func() { close(f.done) }); return nil }
func (f *fakePort) Write(b []byte) (int, error) {
	if f.blockWrite {
		<-f.done
		return 0, io.EOF
	}
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.zeroWrite {
		return 0, nil
	}
	select {
	case <-f.done:
		return 0, io.EOF
	default:
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(b)
	if f.maxWrite > 0 {
		n = min(n, f.maxWrite)
	}
	f.writes = append(f.writes, b[:n]...)
	return n, nil
}
func (f *fakePort) Read(b []byte) (int, error) {
	if f.entered != nil {
		f.onceEntered()
	}
	if f.blocked {
		<-f.done
		return 0, io.EOF
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.chunks) > 0 {
		c := f.chunks[0]
		n := copy(b, c)
		if n == len(c) {
			f.chunks = f.chunks[1:]
		} else {
			f.chunks[0] = c[n:]
		}
		return n, nil
	}
	if f.readErr != nil {
		return 0, f.readErr
	}
	return 0, nil
}
func (f *fakePort) onceEntered() {
	select {
	case f.entered <- struct{}{}:
	default:
	}
}
func TestFragmentedAndAsyncFrames(t *testing.T) {
	f := fake([]byte{10, 56, 2, 10, 22, 35}, []byte{1, 1, 1, 0})
	p, err := newPort(f, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Request(context.Background(), []byte{2, 22})
	if err != nil || !bytes.Equal(got, []byte{10, 22, 35, 1, 1, 1, 0}) {
		t.Fatalf("got %x %v", got, err)
	}
}
func TestUploadWaitsForTerminalAndPreservesRemainder(t *testing.T) {
	progress := append([]byte{10, 33, 0}, make([]byte, 10)...)
	terminal := append([]byte{10, 33, 16}, make([]byte, 10)...)
	stream := append(progress, terminal...)
	stream = append(stream, 10, 24, 2)
	f := fake(stream, []byte{0x12, 0x53})
	p, _ := newPort(f, time.Second, nil)
	got, err := p.Request(context.Background(), nil)
	if err != nil || !bytes.Equal(got, terminal) {
		t.Fatalf("got %x %v", got, err)
	}
	got, err = p.Request(context.Background(), []byte{2, 24})
	if err != nil || !bytes.Equal(got, []byte{10, 24, 2, 0x12, 0x53}) {
		t.Fatalf("remainder %x %v", got, err)
	}
}

// Exact completion bytes from this session's successful physical upload.
func TestObservedUploadCompletionFrame(t *testing.T) {
	frame, err := hex.DecodeString("0a2110a10121a2050300000102")
	if err != nil {
		t.Fatal(err)
	}
	f := fake(frame[:4], frame[4:])
	p, _ := newPort(f, time.Second, nil)
	got, err := p.Request(context.Background(), nil)
	if err != nil || !bytes.Equal(got, frame) {
		t.Fatalf("%x %v", got, err)
	}
}
func TestPartialWritesAndFocusMapping(t *testing.T) {
	response := append([]byte{10, 2}, make([]byte, 40)...)
	f := fake(response)
	f.maxWrite = 1
	p, _ := newPort(f, time.Second, nil)
	got, err := p.Request(context.Background(), []byte{2, 10, 1})
	if err != nil || len(got) != 42 || !bytes.Equal(f.writes, []byte{2, 10, 1}) {
		t.Fatalf("%x %x %v", got, f.writes, err)
	}
}
func TestTimeoutAndCloseInterruptRead(t *testing.T) {
	for _, closeDirect := range []bool{false, true} {
		f := fake()
		f.blocked = true
		f.entered = make(chan struct{}, 1)
		p, _ := newPort(f, time.Second, nil)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() { _, err := p.Request(ctx, []byte{2, 22}); result <- err }()
		<-f.entered
		if closeDirect {
			_ = p.Close()
		} else {
			cancel()
		}
		select {
		case err := <-result:
			if err == nil {
				t.Fatal("missing error")
			}
		case <-time.After(time.Second):
			t.Fatal("read not interrupted")
		}
		cancel()
	}
}
func TestLockWaitCancelable(t *testing.T) {
	f := fake()
	f.blocked = true
	f.entered = make(chan struct{}, 1)
	p, _ := newPort(f, time.Second, nil)
	done := make(chan struct{})
	go func() { _, _ = p.Request(context.Background(), []byte{2, 22}); close(done) }()
	<-f.entered
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := p.Send(ctx, []byte{2, 18, 50}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	_ = p.Close()
	<-done
}
func TestConfigurationAndUnknownAndDisconnect(t *testing.T) {
	f := fake()
	f.timeoutErr = errors.New("set timeout")
	if _, err := newPort(f, time.Second, nil); err == nil {
		t.Fatal("ignored SetReadTimeout")
	}
	f = fake()
	p, _ := newPort(f, time.Second, nil)
	if _, err := p.Request(context.Background(), []byte{2, 0xfe}); err == nil || len(f.writes) != 0 {
		t.Fatal("unknown command written")
	}
	f.readErr = io.EOF
	if _, err := p.Request(context.Background(), []byte{2, 22}); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
func TestBlockedWriteTimeout(t *testing.T) {
	f := fake()
	f.blockWrite = true
	p, _ := newPort(f, 10*time.Millisecond, nil)
	if err := p.Send(context.Background(), []byte{2, 18, 50}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestManyConcurrentSends(t *testing.T) {
	f := fake()
	f.maxWrite = 1
	p, _ := newPort(f, time.Second, nil)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.Send(context.Background(), []byte{2, 18, 50}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(f.writes) != 60 {
		t.Fatalf("writes=%d", len(f.writes))
	}
	for i := 0; i < len(f.writes); i += 3 {
		if !bytes.Equal(f.writes[i:i+3], []byte{2, 18, 50}) {
			t.Fatal("interleaved writes")
		}
	}
}
func TestWriteNoProgress(t *testing.T) {
	f := fake()
	f.zeroWrite = true
	p, _ := newPort(f, time.Second, nil)
	if err := p.Send(context.Background(), []byte{2, 18, 50}); !errors.Is(err, io.ErrNoProgress) {
		t.Fatal(err)
	}
}
func TestUnknownAsyncFailsClosed(t *testing.T) {
	f := fake([]byte{10, 254, 0})
	p, _ := newPort(f, time.Second, nil)
	if _, err := p.Request(context.Background(), []byte{2, 22}); err == nil {
		t.Fatal("unknown frame accepted")
	}
}
func TestOwnershipCheck(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "lsof")
	t.Setenv("PATH", dir)
	for _, tc := range []struct {
		script  string
		wantErr bool
	}{{"#!/bin/sh\nexit 1\n", false}, {"#!/bin/sh\nprintf '1234\\n'\nexit 0\n", true}, {"#!/bin/sh\nprintf 'denied\\n' >&2\nexit 1\n", true}} {
		if err := os.WriteFile(tool, []byte(tc.script), 0700); err != nil {
			t.Fatal(err)
		}
		err := CheckAvailable(context.Background(), "/dev/cu.synthetic")
		if (err != nil) != tc.wantErr {
			t.Fatalf("err=%v expected error=%v", err, tc.wantErr)
		}
	}
}
func TestDefaultTimeout(t *testing.T) {
	f := fake()
	p, _ := newPort(f, 10*time.Millisecond, nil)
	_, err := p.Request(context.Background(), []byte{2, 22})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
