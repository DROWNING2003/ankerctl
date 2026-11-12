package commands

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

// Control uses only the explicitly analyzed real CDC operations.
func Control(ctx context.Context, o Options, operation, value string) error {
	if operation == "delete" {
		return deleteManagedScreen(ctx, o, value)
	}
	return runControl(ctx, o, operation, value, Open)
}
func ParseCRC(value string) (uint16, error) {
	n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(value), "0x"), 16, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("CRC must be nonzero hexadecimal (e.g. a7fa)")
	}
	return uint16(n), nil
}
func controlPacket(op, value string) ([]byte, error) {
	switch op {
	case "reminder":
		p := []byte{2, 46}
		if value != "" {
			s, err := ParseTimerSeconds(value)
			if err != nil {
				return nil, err
			}
			p = append(p, 0, 0, 0, 0)
			binary.LittleEndian.PutUint32(p[2:], s)
		}
		return p, nil
	case "focus-status":
		return []byte{2, 10}, nil
	case "focus-on":
		return []byte{2, 10, 1}, nil
	case "focus-off":
		return []byte{2, 10, 0}, nil
	case "timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			seconds, e := strconv.ParseUint(value, 10, 32)
			if e != nil {
				return nil, fmt.Errorf("invalid timeout duration")
			}
			d = time.Duration(seconds) * time.Second
		}
		for i, v := range []time.Duration{30 * time.Second, time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute, 3 * time.Hour} {
			if d == v {
				return []byte{2, 17, byte(i)}, nil
			}
		}
		return nil, fmt.Errorf("timeout must be 30s, 1m, 5m, 10m, 30m or 3h")
	case "language":
		v, ok := map[string]byte{"en": 0, "zh": 1, "ja": 2, "de": 3}[value]
		if !ok {
			return nil, fmt.Errorf("language must be en, zh, ja or de")
		}
		return []byte{2, 16, v}, nil
	case "screen-on":
		return []byte{2, 31, 1}, nil
	case "screen-off":
		return []byte{2, 31, 0}, nil
	case "select", "delete":
		crc, err := ParseCRC(value)
		if err != nil {
			return nil, err
		}
		cmd := byte(25)
		if op == "delete" {
			cmd = 26
		}
		p := []byte{2, cmd, 0, 0}
		binary.BigEndian.PutUint16(p[2:], crc)
		return p, nil
	}
	return nil, protocol.Unsupported(op)
}
func runControl(ctx context.Context, o Options, op, value string, open func(Options) (serial.Transport, error)) error {
	packet, err := controlPacket(op, value)
	if err != nil {
		return err
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tr, err := open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	var frame []byte
	switch op {
	case "timeout", "language", "screen-on", "screen-off", "select":
		if err := tr.Send(ctx, packet); err != nil {
			return err
		}
		if err := waitGallery(ctx, 300*time.Millisecond); err != nil {
			return err
		}
		query := []byte{2, 22}
		if strings.HasPrefix(op, "screen-") {
			query = []byte{2, 22}
		}
		if op == "select" {
			query = []byte{2, 24}
		}
		frame, err = tr.Request(ctx, query)
	default:
		frame, err = tr.Request(ctx, packet)
	}
	if err != nil {
		return fmt.Errorf("%s outcome not confirmed: %w", op, err)
	}
	var result any
	switch op {
	case "reminder":
		if len(frame) != 6 || frame[0] != 10 || frame[1] != 46 {
			return fmt.Errorf("invalid reminder response")
		}
		s := binary.LittleEndian.Uint32(frame[2:])
		if value != "" && s != binary.LittleEndian.Uint32(packet[2:]) {
			return fmt.Errorf("reminder readback mismatch")
		}
		result = map[string]any{"seconds": s}
	case "focus-status", "focus-on", "focus-off":
		if len(frame) != 42 || frame[0] != 10 || frame[1] != 2 {
			return fmt.Errorf("invalid focus response")
		}
		enabled := frame[2] != 0
		if op != "focus-status" && enabled != (op == "focus-on") {
			return fmt.Errorf("focus switch readback mismatch")
		}
		u := func(offset int) uint32 { return binary.LittleEndian.Uint32(frame[offset:]) }
		idle := uint32(0)
		if u(14) > u(10) {
			idle = u(14) - u(10)
		}
		result = map[string]any{"enabled": enabled, "is_focusing": u(6) > 0, "elapsed_seconds": u(6), "focus_duration_seconds": u(10), "focus_total_seconds": u(14), "idle_seconds": idle, "longest_continuous_seconds": u(18), "total_records": u(22), "cumulative_focus_seconds": u(26), "cumulative_leisure_seconds": u(30), "historical_longest_seconds": u(34)}
	case "timeout", "language":
		s, err := protocol.ParseScreenSettings(frame)
		if err != nil {
			return err
		}
		actual := s.LockTime
		if op == "language" {
			actual = s.Language
		}
		if actual != packet[2] {
			return fmt.Errorf("%s readback mismatch", op)
		}
		result = map[string]any{"operation": op, "value": value, "verified": true}
	case "screen-on", "screen-off":
		settings, err := protocol.ParseScreenSettings(frame)
		if err != nil {
			return err
		}
		if settings.ScreensaverOn != (packet[2] == 1) {
			return fmt.Errorf("screensaver switch readback mismatch")
		}
		result = map[string]any{"enabled": settings.ScreensaverOn, "verified": true}
	case "select":
		s, err := protocol.ParseCurrentScreen(frame)
		if err != nil {
			return err
		}
		crc := binary.BigEndian.Uint16(packet[2:])
		if s == nil || s.CRC != crc {
			return fmt.Errorf("wallpaper selection not confirmed (CRC may not exist)")
		}
		result = map[string]any{"crc16": fmt.Sprintf("%04x", crc), "slot": s.Slot, "verified": true}
	case "delete":
		if len(frame) != 3 || frame[0] != 10 || frame[1] != 26 {
			return fmt.Errorf("invalid delete response; outcome unknown")
		}
		if frame[2] == 1 {
			return ErrWallpaperNotFound
		}
		if frame[2] != 0 {
			return fmt.Errorf("device rejected deletion: status=%d", frame[2])
		}
		result = map[string]any{"deleted": value, "verified": true}
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(o.Out, string(b))
	return err
}
