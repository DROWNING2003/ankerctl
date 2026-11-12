package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
	"time"
)

func TestScreenGetIsStructuredAndExplicitlyPartial(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 24, 3, 0x12, 0x53}, {10, 28, 0, 4, 3, 15, 0, 60}}}
	var out bytes.Buffer
	err := screenGet(context.Background(), Options{Timeout: time.Second, JSON: true, Out: &out}, m)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Complete bool `json:"complete_image_list"`
		Current  struct {
			Slot uint8  `json:"slot"`
			CRC  string `json:"crc16"`
		} `json:"current"`
		Occupancy struct {
			Count uint8 `json:"count"`
			Slots []int `json:"occupied_slots"`
		} `json:"occupancy"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Complete || got.Current.CRC != "1253" || got.Current.Slot != 3 || got.Occupancy.Count != 4 || len(got.Occupancy.Slots) != 4 {
		t.Fatalf("%s", out.String())
	}
	if len(m.Sent) != 2 || !bytes.Equal(m.Sent[0], []byte{2, 24}) || !bytes.Equal(m.Sent[1], []byte{2, 28}) {
		t.Fatalf("TX=%x", m.Sent)
	}
}
func TestScreenGetRejectsInconsistentOccupancy(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 24, 3, 0x12, 0x53}, {10, 28, 0, 7, 3, 15, 0, 60}}}
	if err := screenGet(context.Background(), Options{Out: &bytes.Buffer{}}, m); err == nil {
		t.Fatal("accepted inconsistent occupancy")
	}
}
