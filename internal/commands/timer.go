package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

// ParseTimerSeconds accepts whole seconds or a Go duration such as 25m or 90s.
func ParseTimerSeconds(value string) (uint32, error) {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		d, e := time.ParseDuration(value)
		if e != nil || d < 0 || d%time.Second != 0 || d > time.Hour {
			return 0, fmt.Errorf("invalid timer duration %q: use seconds or a duration such as 25m", value)
		}
		n = uint64(d / time.Second)
	}
	if n < 30 || n > 3600 || n%30 != 0 {
		return 0, fmt.Errorf("duration must be 30..3600 seconds and a multiple of 30")
	}
	return uint32(n), nil
}
func Timer(ctx context.Context, o Options, operation string, value string) error {
	return runTimer(ctx, o, operation, value, Open)
}
func runTimer(ctx context.Context, o Options, operation string, value string, open func(Options) (serial.Transport, error)) error {
	var request []byte
	switch operation {
	case "duration":
		request = []byte{2, 0x2a}
		if value != "" {
			seconds, err := ParseTimerSeconds(value)
			if err != nil {
				return err
			}
			request, _ = protocol.TimerDurationRequest(seconds)
		}
	case "status":
		request = []byte{2, 0x2b}
	case "start":
		request = []byte{2, 0x2c}
	case "stats":
		request = []byte{2, 0x2d}
	case "stop":
		return protocol.Unsupported("timer stop: no stop command in analyzed firmware client")
	default:
		return protocol.Unsupported("timer " + operation)
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
	frame, err := tr.Request(ctx, request)
	if err != nil {
		if operation == "start" {
			return fmt.Errorf("timer start outcome unknown; query timer status before retrying: %w", err)
		}
		return err
	}
	var result any
	switch operation {
	case "duration":
		seconds, err := protocol.ParseTimerDuration(frame)
		if err != nil {
			return err
		}
		if value != "" {
			expected, _ := ParseTimerSeconds(value)
			if expected != seconds {
				return fmt.Errorf("duration readback mismatch: expected %d seconds, received %d", expected, seconds)
			}
		}
		result = struct {
			Seconds uint32 `json:"seconds"`
		}{seconds}
	case "status":
		v, err := protocol.ParseTimerStatus(frame)
		if err != nil {
			return err
		}
		result = v
	case "stats":
		v, err := protocol.ParseTimerStats(frame)
		if err != nil {
			return err
		}
		result = v
	case "start":
		if err := protocol.ParseTimerStart(frame); err != nil {
			return fmt.Errorf("timer start not confirmed; query timer status before retrying: %w", err)
		}
		result = struct {
			Accepted bool `json:"accepted"`
		}{true}
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	switch v := result.(type) {
	case protocol.TimerStatus:
		fmt.Fprintf(o.Out, "state=%d remaining=%ds completion_seq=%d\n", v.State, v.RemainingSeconds, v.CompletionSeq)
	case protocol.TimerStats:
		fmt.Fprintf(o.Out, "completed=%d total=%ds\n", v.CompletedCount, v.CompletedSeconds)
	default:
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Fprintln(o.Out, string(b))
	}
	return nil
}
