package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/device"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/DROWNING2003/ankerctl/internal/serial"
)

type Options struct {
	Context context.Context
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

func optionContext(o Options) context.Context {
	if o.Context != nil {
		return o.Context
	}
	return context.Background()
}

func readOnly(o Options, operation string, request []byte) error {
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	return readOnlyTransport(ctx, o, operation, request, tr)
}
func readOnlyTransport(ctx context.Context, o Options, operation string, request []byte, tr serial.Transport) error {
	resp, err := tr.Request(ctx, request)
	if err != nil {
		return err
	}
	if operation != "display get" {
		return protocol.Unsupported(operation)
	}
	if len(resp) != 7 {
		return fmt.Errorf("invalid screen settings response length")
	}
	settings, err := protocol.ParseScreenSettings(resp)
	if err != nil {
		return err
	}
	if resp[5] > 1 {
		return fmt.Errorf("invalid screensaver switch value")
	}
	result := map[string]any{"brightness": settings.Brightness, "lock_time_enum": settings.LockTime, "language_enum": settings.Language, "screensaver_on": settings.ScreensaverOn}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	_, err = fmt.Fprintf(o.Out, "brightness=%d lock_time_enum=%d language_enum=%d screensaver_on=%t\n", settings.Brightness, settings.LockTime, settings.Language, settings.ScreensaverOn)
	return err
}
func Info(o Options) error {
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
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
func ScreenGet(o Options) error {
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	return screenGet(ctx, o, tr)
}
func screenGet(ctx context.Context, o Options, tr serial.Transport) error {
	frame, err := tr.Request(ctx, protocol.ScreenCurrentRequest())
	if err != nil {
		return err
	}
	if len(frame) != 5 {
		return fmt.Errorf("invalid current wallpaper response length")
	}
	current, err := protocol.ParseCurrentScreen(frame)
	if err != nil {
		return err
	}
	frame, err = tr.Request(ctx, protocol.CarouselStatusRequest())
	if err != nil {
		return err
	}
	state, err := protocol.ParseCarouselStatus(frame)
	if err != nil {
		return err
	}
	if state.Count > 8 || int(state.Count) != bits.OnesCount8(state.Bitmap) {
		return fmt.Errorf("inconsistent wallpaper count and occupancy bitmap")
	}
	slots := make([]int, 0, 8)
	for slot := range 8 {
		if state.Bitmap&(1<<slot) != 0 {
			slots = append(slots, slot)
		}
	}
	type currentImage struct {
		Slot uint8  `json:"slot"`
		CRC  string `json:"crc16"`
	}
	var item *currentImage
	if current != nil {
		if current.Slot > 7 || state.Bitmap&(1<<current.Slot) == 0 {
			return fmt.Errorf("current wallpaper is absent from occupancy snapshot; retry query")
		}
		item = &currentImage{current.Slot, fmt.Sprintf("%04x", current.CRC)}
	}
	result := map[string]any{"complete_image_list": false, "scope": "current wallpaper and slot occupancy only; per-image CRCs unavailable", "current": item, "occupancy": map[string]any{"count": state.Count, "bitmap": state.Bitmap, "occupied_slots": slots}, "carousel": state}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(result)
	}
	if item == nil {
		fmt.Fprintln(o.Out, "Current wallpaper: none")
	} else {
		fmt.Fprintf(o.Out, "Current wallpaper: slot=%d crc16=%s\n", item.Slot, item.CRC)
	}
	_, err = fmt.Fprintf(o.Out, "Stored images: %d; occupied slots: %v\nThis is not a complete image list; only the current image CRC is available.\n", state.Count, slots)
	return err
}
func Ports(o Options) error {
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if err := optionContext(o).Err(); err != nil {
		return err
	}
	tr, err := Open(o)
	if err != nil {
		return err
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
	defer cancel()
	resp, err := tr.Request(ctx, protocol.PowerSnapshotRequest())
	if err != nil {
		return err
	}
	s, err := protocol.ParsePowerSnapshot(resp)
	if err != nil {
		return fmt.Errorf("invalid power response: %w", err)
	}
	if o.JSON {
		return json.NewEncoder(o.Out).Encode(s)
	}
	for _, channel := range []struct {
		name   string
		sample protocol.PowerSample
	}{{"UFP ADC", s.UFP}, {"PD ADC", s.PDADC}, {"PD input", s.PDIn}, {"UFP negotiated", s.UFPNeg}} {
		if _, err := fmt.Fprintf(o.Out, "%s: %dmV %dmA %dmW\n", channel.name, channel.sample.VoltageMv, channel.sample.CurrentMa, channel.sample.PowerMw); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(o.Out, "ADC polarity and low-voltage readings do not establish port connection state.")
	return err
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
	ctx, cancel := context.WithTimeout(optionContext(o), o.Timeout)
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
