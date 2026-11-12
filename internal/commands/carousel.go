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

// ParseCarouselSeconds accepts integer seconds or whole-second Go durations.
func ParseCarouselSeconds(value string) (uint16, error) {
	seconds, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		duration, e := time.ParseDuration(value)
		if e != nil || duration < 3*time.Second || duration > time.Hour || duration%time.Second != 0 {
			return 0, fmt.Errorf("carousel interval must be 3..3600 whole seconds (e.g. 60 or 1m)")
		}
		seconds = uint64(duration / time.Second)
	}
	if seconds < 3 || seconds > 3600 {
		return 0, fmt.Errorf("carousel interval must be 3..3600 whole seconds")
	}
	return uint16(seconds), nil
}

// Carousel only controls existing stored images; it never clears or uploads them.
func Carousel(ctx context.Context, o Options, operation, value string) error {
	return runCarousel(ctx, o, operation, value, Open)
}
func runCarousel(ctx context.Context, o Options, operation, value string, open func(Options) (serial.Transport, error)) error {
	var request []byte
	var expectedSeconds uint16
	switch operation {
	case "status":
		request = protocol.CarouselStatusRequest()
	case "on", "off":
		request = protocol.CarouselModeRequest(operation == "on")
	case "interval":
		seconds, err := ParseCarouselSeconds(value)
		if err != nil {
			return err
		}
		expectedSeconds = seconds
		request, err = protocol.CarouselIntervalRequest(seconds)
		if err != nil {
			return err
		}
	default:
		return protocol.Unsupported("screen carousel " + operation)
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
	if operation == "on" {
		if err := tr.Send(ctx, protocol.CarouselEnableScreensaverRequest()); err != nil {
			return fmt.Errorf("screensaver enable not confirmed; query device state before retrying: %w", err)
		}
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return fmt.Errorf("screensaver switch sent but carousel enable not confirmed: %w", ctx.Err())
		case <-timer.C:
		}
	}
	frame, err := tr.Request(ctx, request)
	if err != nil {
		if operation != "status" {
			return fmt.Errorf("carousel change outcome unknown; query carousel status before retrying: %w", err)
		}
		return err
	}
	var result any
	switch operation {
	case "status":
		state, err := protocol.ParseCarouselStatus(frame)
		if err != nil {
			return err
		}
		result = state
	case "on", "off":
		enabled, err := protocol.ParseCarouselMode(frame)
		if err != nil {
			return fmt.Errorf("carousel mode not confirmed; query carousel status: %w", err)
		}
		if enabled != (operation == "on") {
			return fmt.Errorf("carousel mode mismatch: requested enabled=%t, device reported enabled=%t", operation == "on", enabled)
		}
		result = struct {
			Enabled bool `json:"enabled"`
		}{enabled}
	case "interval":
		seconds, err := protocol.ParseCarouselInterval(frame)
		if err != nil {
			return err
		}
		if seconds != expectedSeconds {
			return fmt.Errorf("carousel interval mismatch: requested %d seconds, device reported %d", expectedSeconds, seconds)
		}
		result = struct {
			IntervalSeconds uint16 `json:"interval_seconds"`
		}{seconds}
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	switch state := result.(type) {
	case protocol.CarouselState:
		_, err = fmt.Fprintf(o.Out, "enabled=%t count=%d slot=%d bitmap=0x%02x interval=%ds\n", state.Enabled, state.Count, state.Slot, state.Bitmap, state.IntervalSeconds)
	default:
		encoded, e := json.Marshal(result)
		if e != nil {
			return e
		}
		_, err = fmt.Fprintln(o.Out, string(encoded))
	}
	return err
}
