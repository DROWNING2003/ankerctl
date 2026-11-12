package protocol

import (
	"encoding/binary"
	"fmt"
)

type PowerSample struct {
	VoltageMv uint16
	CurrentMa int16
	PowerMw   int
}
type PortStatus struct {
	UFP, PDIn           PowerSample
	PDLinked, UFPLinked bool
	RawLen              int
}

func ParsePortInfo(frame []byte) (PortStatus, error) {
	if len(frame) < 16 || frame[0] != 0x0a || frame[1] != 0x00 {
		return PortStatus{}, fmt.Errorf("invalid 0A00 response")
	}
	if len(frame) < 64 {
		return PortStatus{RawLen: len(frame)}, nil
	}
	flags := frame[3]
	u := PowerSample{binary.BigEndian.Uint16(frame[56:58]), int16(binary.BigEndian.Uint16(frame[58:60])), 0}
	p := PowerSample{binary.BigEndian.Uint16(frame[60:62]), int16(binary.BigEndian.Uint16(frame[62:64])), 0}
	u.PowerMw = int(int(u.VoltageMv) * int(abs16(u.CurrentMa)) / 1000)
	p.PowerMw = int(int(p.VoltageMv) * int(p.CurrentMa) / 1000)
	return PortStatus{UFP: u, PDIn: p, PDLinked: flags&2 != 0, UFPLinked: flags&4 != 0, RawLen: len(frame)}, nil
}
func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
func PortInfoRequest() []byte      { return []byte{2, 0} }
func PowerSnapshotRequest() []byte { return []byte{2, 57} }
func ParsePowerSnapshot(frame []byte) (PortStatus, error) {
	if len(frame) < 16 || frame[0] != 0x0a || frame[1] != 0x39 {
		return PortStatus{}, fmt.Errorf("invalid 0A39 response")
	}
	sample := func(offV, offI int) PowerSample {
		v := binary.BigEndian.Uint16(frame[offV:])
		i := int16(binary.BigEndian.Uint16(frame[offI:]))
		return PowerSample{v, i, int(int(v) * int(i) / 1000)}
	}
	return PortStatus{UFP: sample(2, 4), PDIn: sample(6, 8), PDLinked: binary.BigEndian.Uint16(frame[6:8]) > 0, UFPLinked: binary.BigEndian.Uint16(frame[2:4]) > 0, RawLen: len(frame)}, nil
}
