package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
	"time"
)

func TestStatusSectionsPreservePartialFailures(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{
		{10, 56, 2}, append([]byte{10, 48}, make([]byte, 16)...), // firmware
		{10, 57}, // malformed power only
		append([]byte{10, 0}, make([]byte, 62)...),
		{10, 39, 0, 0, 0, 0, 0, 0, 0},
		{10, 22, 30, 1, 1, 1, 0},
		{10, 24, 3, 0x12, 0x53}, {10, 28, 0, 4, 3, 15, 0, 60},
		{10, 43, 1, 30, 0, 0, 0, 0, 0, 0, 0},
		{10, 46, 44, 1, 0, 0},
	}}
	var out bytes.Buffer
	err := statusWithTransport(context.Background(), Options{Timeout: time.Second, JSON: true, Out: &out}, m)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]StatusSection
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["power"].Error == "" || result["firmware"].Error != "" || result["timer"].Data == nil || result["reminder"].Data == nil {
		t.Fatalf("%s", out.String())
	}
}
