package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/DROWNING2003/ankerctl/internal/serial"
	"testing"
)

func TestDisplayReadOnlyRejectsMalformedResponse(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 22, 30}}}
	if err := readOnlyTransport(context.Background(), Options{Out: &bytes.Buffer{}}, "display get", []byte{2, 22}, m); err == nil {
		t.Fatal("incomplete read returned success")
	}
}
func TestDisplayReadOnlyJSONSettings(t *testing.T) {
	m := &serial.MockTransport{Responses: [][]byte{{10, 22, 30, 1, 1, 1, 0}}}
	var out bytes.Buffer
	err := readOnlyTransport(context.Background(), Options{Out: &out, JSON: true}, "display get", []byte{2, 22}, m)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["brightness"] != float64(30) || result["screensaver_on"] != true {
		t.Fatalf("%s", out.String())
	}
}
