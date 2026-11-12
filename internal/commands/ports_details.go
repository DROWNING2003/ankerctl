package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

// DetailedPorts reads connection/speed/display fields from 0200 and a separate
// power snapshot from 0239. These sequential observations are not atomic.
func DetailedPorts(ctx context.Context, o Options) error {
	return runPortsDetails(ctx, o, "ports", Open)
}

// Displays combines 0200 resolution/EDID with 0227 physical diagonal sizes.
func Displays(ctx context.Context, o Options) error { return runPortsDetails(ctx, o, "displays", Open) }

type DetailedPortsResult struct {
	Schema string                    `json:"schema"`
	Ports  protocol.DetailedPortInfo `json:"ports"`
	Power  protocol.PortStatus       `json:"power_snapshot"`
	Atomic bool                      `json:"atomic_snapshot"`
}
type DisplayResult struct {
	Schema        string                 `json:"schema"`
	DP            protocol.DisplayPort   `json:"dp"`
	HDMI          protocol.DisplayPort   `json:"hdmi"`
	PhysicalSizes protocol.PhysicalSizes `json:"physical_sizes"`
	Atomic        bool                   `json:"atomic_snapshot"`
}

func runPortsDetails(ctx context.Context, o Options, op string, open func(Options) (serial.Transport, error)) error {
	if op != "ports" && op != "displays" {
		return protocol.Unsupported(op)
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	o.Context = ctx
	tr, err := open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	frame, err := tr.Request(ctx, protocol.PortInfoRequest())
	if err != nil {
		return fmt.Errorf("read port info: %w", err)
	}
	details, err := protocol.ParseDetailedPortInfo(frame)
	if err != nil {
		return err
	}
	var result any
	if op == "ports" {
		frame, err = tr.Request(ctx, protocol.PowerSnapshotRequest())
		if err != nil {
			return fmt.Errorf("read power snapshot: %w", err)
		}
		power, err := protocol.ParsePowerSnapshot(frame)
		if err != nil {
			return err
		}
		result = DetailedPortsResult{Schema: "ankerctl.ports.v1", Ports: details, Power: power}
	} else {
		frame, err = tr.Request(ctx, protocol.ScreenSizeRequest())
		if err != nil {
			return fmt.Errorf("read physical display sizes: %w", err)
		}
		sizes, err := protocol.ParsePhysicalSizes(frame)
		if err != nil {
			return err
		}
		// Match official presentation: disconnected video outputs do not acquire a
		// display size just because an older size response retained data.
		if !details.DP.Connected {
			sizes.DP = protocol.PhysicalSize{}
		}
		if !details.HDMI.Connected {
			sizes.HDMI = protocol.PhysicalSize{}
		}
		result = DisplayResult{Schema: "ankerctl.displays.v1", DP: details.DP, HDMI: details.HDMI, PhysicalSizes: sizes}
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	// Indented, named fields keep unknown/omitted values distinct from invented data.
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(o.Out, string(b))
	return err
}
