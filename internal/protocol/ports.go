package protocol

import (
	"encoding/binary"
	"fmt"
)

// CurrentMa is widened to int32 so signed ADC and unsigned PD-input fields
// retain their full ranges. PowerMw is signed for ADC samples.
type PowerSample struct {
	VoltageMv uint16 `json:"voltage_mv"`
	CurrentMa int32  `json:"current_ma"`
	PowerMw   int64  `json:"power_mw"`
}
type PortStatus struct {
	UFP    PowerSample `json:"ufp_adc"`
	PDADC  PowerSample `json:"pd_adc"`
	PDIn   PowerSample `json:"pd_in"`
	UFPNeg PowerSample `json:"ufp_negotiated"`
	// Only ParsePortInfo supplies independent link flags. Snapshot ADC voltages
	// are not evidence of a connected port.
	PDLinked  bool `json:"-"`
	UFPLinked bool `json:"-"`
	RawLen    int  `json:"-"`
}

func powerSample(v uint16, i int32) PowerSample { return PowerSample{v, i, int64(v) * int64(i) / 1000} }
func ParsePortInfo(frame []byte) (PortStatus, error) {
	if len(frame) != 64 || frame[0] != 10 || frame[1] != 0 {
		return PortStatus{}, fmt.Errorf("invalid 0A00 response: expected 64 bytes")
	}
	u := powerSample(binary.BigEndian.Uint16(frame[54:]), int32(int16(binary.BigEndian.Uint16(frame[56:]))))
	// Official 0A00 presentation reports power magnitude; widen before abs so
	// -32768 mA does not overflow int16.
	if u.PowerMw < 0 {
		u.PowerMw = -u.PowerMw
	}
	p := powerSample(binary.BigEndian.Uint16(frame[58:]), int32(binary.BigEndian.Uint16(frame[60:])))
	return PortStatus{UFP: u, PDIn: p, UFPNeg: powerSample(uint16(frame[62])*100, int32(frame[63])*50), PDLinked: frame[3]&2 != 0, UFPLinked: frame[3]&4 != 0, RawLen: len(frame)}, nil
}
func PortInfoRequest() []byte      { return []byte{2, 0} }
func PowerSnapshotRequest() []byte { return []byte{2, 57} }
func ParsePowerSnapshot(frame []byte) (PortStatus, error) {
	if len(frame) != 16 || frame[0] != 10 || frame[1] != 0x39 {
		return PortStatus{}, fmt.Errorf("invalid 0A39 response: expected 16 bytes")
	}
	signed := func(v, i int) PowerSample {
		return powerSample(binary.BigEndian.Uint16(frame[v:]), int32(int16(binary.BigEndian.Uint16(frame[i:]))))
	}
	return PortStatus{UFP: signed(2, 4), PDADC: signed(6, 8), PDIn: powerSample(binary.BigEndian.Uint16(frame[10:]), int32(binary.BigEndian.Uint16(frame[12:]))), UFPNeg: powerSample(uint16(frame[14])*100, int32(frame[15])*50), RawLen: len(frame)}, nil
}
