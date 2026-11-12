package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"ankerctl/internal/device"
	"ankerctl/internal/protocol"
	"ankerctl/internal/serial"
)

type Options struct {
	Port    string
	JSON    bool
	Timeout time.Duration
	Debug   bool
	Out     io.Writer
	Err     io.Writer
}

func Devices(o Options) error {
	ps, err := device.ListPorts()
	if err != nil {
		return err
	}
	if o.JSON {
		public := make([]device.PublicPort, 0, len(ps))
		for _, p := range ps {
			public = append(public, p.Public())
		}
		return json.NewEncoder(o.Out).Encode(public)
	}
	for _, p := range ps {
		fmt.Fprintf(o.Out, "%s VID=%s PID=%s manufacturer=%s A210C=%t\n", p.Path, p.VID, p.PID, p.Manufacturer, p.IsA210C)
	}
	return nil
}
func Unsupported(o Options, name string) error {
	if !o.JSON {
		fmt.Fprintf(o.Out, "Unsupported: %s\n", name)
	}
	return protocol.Unsupported(name)
}
func Open(o Options) (serial.Transport, error) {
	port := o.Port
	if port == "" {
		ports, err := device.ListPorts()
		if err != nil {
			return nil, err
		}
		var matches []device.Port
		for _, p := range ports {
			if p.IsA210C {
				matches = append(matches, p)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no Anker A210C serial port found; use --port to select one")
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("multiple Anker A210C serial ports found; use --port to select one")
		}
		port = matches[0].Path
		if !o.JSON {
			fmt.Fprintf(o.Err, "using Anker A210C port: %s\n", port)
		}
	}
	if err := device.Check(context.Background(), port); err != nil {
		return nil, err
	}
	return serial.Open(port, o.Timeout, func(direction string, b []byte) {
		if o.Debug {
			fmt.Fprintf(o.Err, "%s %x\n", direction, b)
		}
	})
}

func readOnly(o Options, operation string, request []byte) error {
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	resp, err := tr.Request(ctx, request)
	if err != nil {
		return err
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(map[string]any{"operation": operation, "raw": fmt.Sprintf("%x", resp)})
	}
	fmt.Fprintf(o.Out, "%s response: %x\n", operation, resp)
	return nil
}
func Info(o Options) error {
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	if _, err := tr.Request(ctx, []byte{2, 56, 2}); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	resp, err := tr.Request(ctx, []byte{2, 48})
	if err != nil {
		return err
	}
	if v, e := protocol.ParseFirmwareVersion(resp); e == nil {
		if o.JSON {
			return json.NewEncoder(o.Out).Encode(v)
		}
		fmt.Fprintf(o.Out, "MCU=%s UI=%s\n", v.MCU, v.UI)
		return nil
	}
	return fmt.Errorf("invalid firmware response: %x", resp)
}
func DisplayGet(o Options) error { return readOnly(o, "display get", protocol.ScreenSettingsRequest()) }
func ScreenGet(o Options) error  { return readOnly(o, "screen get", protocol.ScreenCurrentRequest()) }
func Ports(o Options) error {
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	resp, err := tr.Request(ctx, protocol.PowerSnapshotRequest())
	if err != nil {
		return err
	}
	s, err := protocol.ParsePowerSnapshot(resp)
	if err != nil {
		return fmt.Errorf("invalid power response: %w", err)
	}
	out := map[string]any{"ufp_voltage_mv": s.UFP.VoltageMv, "ufp_current_ma": s.UFP.CurrentMa, "ufp_power_mw": s.UFP.PowerMw, "pd_voltage_mv": s.PDIn.VoltageMv, "pd_current_ma": s.PDIn.CurrentMa, "pd_power_mw": s.PDIn.PowerMw, "ufp_linked": s.UFPLinked, "pd_linked": s.PDLinked}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(out)
	}
	fmt.Fprintf(o.Out, "UFP: %dmV %dmA %dmW linked=%t\nPD in: %dmV %dmA %dmW linked=%t\n", s.UFP.VoltageMv, s.UFP.CurrentMa, s.UFP.PowerMw, s.UFPLinked, s.PDIn.VoltageMv, s.PDIn.CurrentMa, s.PDIn.PowerMw, s.PDLinked)
	return nil
}
func Brightness(o Options, value int) error {
	frame, err := protocol.SetBrightness(value)
	if err != nil {
		return err
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	if err := tr.Send(ctx, frame); err != nil {
		return err
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(map[string]any{"operation": "display brightness", "brightness": value, "status": "sent"})
	}
	fmt.Fprintf(o.Out, "brightness command sent: %d\n", value)
	return nil
}
