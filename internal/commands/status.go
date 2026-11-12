package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

// Each section is independently observed; no fallback values impersonate readings.
type StatusSection struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func Status(ctx context.Context, o Options) error {
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	return statusWithTransport(ctx, o, tr)
}
func statusWithTransport(ctx context.Context, o Options, tr serial.Transport) error {
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	sections := map[string]StatusSection{}
	// One connection and sequential queries, bounded per section by --timeout.
	queries := []struct {
		name string
		run  func(context.Context) (any, error)
	}{
		{"firmware", func(ctx context.Context) (any, error) {
			if _, err := tr.Request(ctx, []byte{2, 56, 2}); err != nil {
				return nil, err
			}
			f, err := tr.Request(ctx, []byte{2, 48})
			if err != nil {
				return nil, err
			}
			return protocol.ParseFirmwareVersion(f)
		}},
		{"power", func(ctx context.Context) (any, error) {
			f, err := tr.Request(ctx, []byte{2, 57})
			if err != nil {
				return nil, err
			}
			return protocol.ParsePowerSnapshot(f)
		}},
		{"ports", func(ctx context.Context) (any, error) {
			f, err := tr.Request(ctx, protocol.PortInfoRequest())
			if err != nil {
				return nil, err
			}
			return protocol.ParseDetailedPortInfo(f)
		}},
		{"physical_display_sizes", func(ctx context.Context) (any, error) {
			f, err := tr.Request(ctx, protocol.ScreenSizeRequest())
			if err != nil {
				return nil, err
			}
			return protocol.ParsePhysicalSizes(f)
		}},
		{"display", func(ctx context.Context) (any, error) {
			f, err := tr.Request(ctx, []byte{2, 22})
			if err != nil {
				return nil, err
			}
			s, err := protocol.ParseScreenSettings(f)
			if err != nil {
				return nil, err
			}
			return map[string]any{"brightness": s.Brightness, "lock_time_enum": s.LockTime, "language_enum": s.Language, "screensaver_on": s.ScreensaverOn}, nil
		}},
		{"wallpaper", func(ctx context.Context) (any, error) {
			var out bytes.Buffer
			options := o
			options.Out = &out
			options.JSON = true
			if err := screenGet(ctx, options, tr); err != nil {
				return nil, err
			}
			var result any
			err := json.Unmarshal(out.Bytes(), &result)
			return result, err
		}},
		{"timer", func(ctx context.Context) (any, error) {
			f, err := tr.Request(ctx, []byte{2, 43})
			if err != nil {
				return nil, err
			}
			return protocol.ParseTimerStatus(f)
		}},
		{"reminder", func(ctx context.Context) (any, error) {
			var out bytes.Buffer
			options := o
			options.Out = &out
			options.JSON = true
			err := runControl(ctx, options, "reminder", "", func(Options) (serial.Transport, error) { return noCloseTransport{tr}, nil })
			if err != nil {
				return nil, err
			}
			var result any
			err = json.Unmarshal(out.Bytes(), &result)
			return result, err
		}},
	}
	for _, query := range queries {
		if err := ctx.Err(); err != nil {
			return err
		}
		sectionCtx, cancel := context.WithTimeout(ctx, o.Timeout)
		data, err := query.run(sectionCtx)
		cancel()
		if err != nil {
			sections[query.name] = StatusSection{Error: err.Error()}
		} else {
			sections[query.name] = StatusSection{Data: data}
		}
		if err := waitGallery(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
	if o.JSON {
		return writeJSON(o.Out, sections)
	}
	b, err := json.MarshalIndent(sections, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(o.Out, string(b))
	return err
}
