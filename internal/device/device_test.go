package device

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicDeviceDoesNotLeakSerialOrFakeManufacturer(t *testing.T) {
	p := Port{Path: "/dev/test", Product: "A210C", SerialNumber: "secret-device-serial", IsA210C: true}
	b, err := json.Marshal(p.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), p.SerialNumber) {
		t.Fatal("serial leak")
	}
	if p.Public().Manufacturer != "" || p.Public().Product != "A210C" {
		t.Fatal("product confused with manufacturer")
	}
}
