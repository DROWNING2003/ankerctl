package serial

import (
	"context"
	"sync"
)

type MockTransport struct {
	mu        sync.Mutex
	Responses [][]byte
	Sent      [][]byte
	Err       error
	Closed    bool
}

func (m *MockTransport) Request(ctx context.Context, tx []byte) ([]byte, error) {
	if err := m.Send(ctx, tx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	if len(m.Responses) == 0 {
		return nil, nil
	}
	r := append([]byte(nil), m.Responses[0]...)
	m.Responses = m.Responses[1:]
	return r, nil
}
func (m *MockTransport) Send(ctx context.Context, tx []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, append([]byte(nil), tx...))
	return m.Err
}
func (m *MockTransport) Close() error { m.mu.Lock(); defer m.mu.Unlock(); m.Closed = true; return nil }
