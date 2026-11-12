package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/serial"
)

func TestDetailedPortsRequestsAndJSON(t *testing.T) {
	for _, op := range []string{"ports", "displays"} {
		t.Run(op, func(t *testing.T) {
			f := make([]byte, 64)
			f[0] = 10
			f[3] = 1
			f[9] = 4
			second := []byte{10, 57, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			want := []byte{2, 57}
			if op == "displays" {
				second = []byte{10, 39, 0, 0, 0, 0, 0, 0, 0}
				want = []byte{2, 39}
			}
			m := &serial.MockTransport{Responses: [][]byte{f, second}}
			var out bytes.Buffer
			err := runPortsDetails(context.Background(), Options{Timeout: time.Second, JSON: true, Out: &out}, op, func(Options) (serial.Transport, error) { return m, nil })
			if err != nil {
				t.Fatal(err)
			}
			if !m.Closed || len(m.Sent) != 2 || !bytes.Equal(m.Sent[0], []byte{2, 0}) || !bytes.Equal(m.Sent[1], want) {
				t.Fatalf("TX=%x closed=%v", m.Sent, m.Closed)
			}
			var got map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got["schema"] != "ankerctl."+op+".v1" || got["atomic_snapshot"] != false {
				t.Fatal(got)
			}
			if op == "ports" {
				ports := got["ports"].(map[string]any)
				if ports["ethernet_linked"] != true {
					t.Fatal(got)
				}
				usb := ports["usb"].(map[string]any)
				if usb["c1"].(map[string]any)["speed"] != "10 Gbps" {
					t.Fatal(got)
				}
				if _, ok := ports["port_power"].(map[string]any)["pd_adc"]; ok {
					t.Fatal("0A00 invented PD ADC")
				}
				if _, ok := got["power_snapshot"].(map[string]any)["pd_adc"]; !ok {
					t.Fatal("missing snapshot ADC")
				}
			}
		})
	}
}
func TestPortsDetailsFailures(t *testing.T) {
	for _, responses := range [][][]byte{{{10, 0}}, {append([]byte{10, 0}, make([]byte, 62)...), {10, 57}}} {
		m := &serial.MockTransport{Responses: responses}
		err := runPortsDetails(context.Background(), Options{Timeout: time.Second}, "ports", func(Options) (serial.Transport, error) { return m, nil })
		if err == nil || !m.Closed {
			t.Fatalf("err=%v closed=%v", err, m.Closed)
		}
	}
}
func TestPortsDetailsValidationBeforeOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx     context.Context
		timeout time.Duration
	}{{ctx, time.Second}, {context.Background(), 0}} {
		err := runPortsDetails(tc.ctx, Options{Timeout: tc.timeout}, "ports", func(Options) (serial.Transport, error) { t.Fatal("unexpected open"); return nil, nil })
		if err == nil {
			t.Fatal("expected error")
		}
	}
}
func TestPortsDetailsOpenAndRequestErrors(t *testing.T) {
	sentinel := errors.New("unavailable")
	if err := runPortsDetails(context.Background(), Options{Timeout: time.Second}, "ports", func(Options) (serial.Transport, error) { return nil, sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	m := &serial.MockTransport{Err: sentinel}
	if err := runPortsDetails(context.Background(), Options{Timeout: time.Second}, "displays", func(Options) (serial.Transport, error) { return m, nil }); !errors.Is(err, sentinel) || !m.Closed {
		t.Fatal(err)
	}
}
