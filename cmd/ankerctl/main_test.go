package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONErrorsAreSingleObjects(t *testing.T) {
	for _, args := range [][]string{{"--json", "display", "brightness", "9"}, {"--json", "screen", "delete", "abcd"}, {"--json", "unknown"}, {"--json", "screen", "unknown"}, {"--json", "timer", "unknown"}, {"--json", "devices", "extra"}, {"--json", "--badflag"}, {"--json", "--timeout", "0", "ports"}} {
		var out, stderr bytes.Buffer
		code := run(context.Background(), args, &out, &stderr)
		if code == 0 {
			t.Fatalf("success for %v", args)
		}
		var response struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.Error.Message == "" || response.Error.Code == "" {
			t.Fatalf("%v out=%q err=%v", args, out.String(), err)
		}
		if strings.Contains(out.String(), "Usage:") || stderr.Len() != 0 {
			t.Fatalf("mixed output: %q %q", out.String(), stderr.String())
		}
	}
}
func TestHelpAndNoArgsValidation(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{"display", "brightness", "--help"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "<10-100>") {
		t.Fatalf("%d %s", code, out.String())
	}
	out.Reset()
	if code := run(context.Background(), []string{"devices", "extra"}, &out, &stderr); code == 0 {
		t.Fatal("accepted extra args")
	}
}
func TestRootOptionsAreIndependent(t *testing.T) {
	var out, stderr bytes.Buffer
	r1 := newRoot(&out, &stderr)
	r1.SetArgs([]string{"--json", "timer", "stop"})
	_ = r1.Execute()
	r2 := newRoot(&out, &stderr)
	value, _ := r2.PersistentFlags().GetBool("json")
	if value {
		t.Fatal("options leaked across roots")
	}
}
func TestCanceledContextNeverRunsDeviceCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, stderr bytes.Buffer
	if code := run(ctx, []string{"--json", "ports"}, &out, &stderr); code != 130 {
		t.Fatalf("got %d", code)
	}
}
