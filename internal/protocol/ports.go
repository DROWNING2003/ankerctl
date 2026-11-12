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

// USBPort exposes the raw speed bit mask; unknown flags never become a guessed speed.
type USBPort struct {
	Connected bool   `json:"connected"`
	RawFlags  uint8  `json:"raw_flags"`
	Speed     string `json:"speed,omitempty"`
}
type USBPorts struct {
	SDTF USBPort `json:"sd_tf"`
	C1   USBPort `json:"c1"`
	A1   USBPort `json:"a1"`
	A2   USBPort `json:"a2"`
	A3   USBPort `json:"a3"`
}
type DisplayPort struct {
	Connected    bool   `json:"connected"`
	Width        uint16 `json:"width_pixels,omitempty"`
	Height       uint16 `json:"height_pixels,omitempty"`
	RefreshHz    *uint8 `json:"refresh_hz,omitempty"`
	RawRefreshHz uint8  `json:"raw_refresh_hz"`
	EDIDPresent  bool   `json:"edid_present"`
	Manufacturer string `json:"manufacturer,omitempty"`
	ProductCode  string `json:"product_code,omitempty"`
	Name         string `json:"name,omitempty"`
}

// PortPower excludes PD ADC: 0A00 has no independent PD-ADC field.
type PortPower struct {
	UFP    PowerSample `json:"ufp_adc"`
	PDIn   PowerSample `json:"pd_in"`
	UFPNeg PowerSample `json:"ufp_negotiated"`
}
type DetailedPortInfo struct {
	USB              USBPorts    `json:"usb"`
	DP               DisplayPort `json:"dp"`
	HDMI             DisplayPort `json:"hdmi"`
	EthernetLinked   bool        `json:"ethernet_linked"`
	PDLinked         bool        `json:"pd_linked"`
	UFPLinked        bool        `json:"ufp_linked"`
	HostVendorIndex  uint16      `json:"host_vendor_index"`
	HostProductIndex uint16      `json:"host_product_index"`
	RawLinkFlags     uint8       `json:"raw_link_flags"`
	Power            PortPower   `json:"port_power"`
}
type PhysicalSize struct {
	ValidFlag      bool     `json:"valid_flag"`
	DiagonalInches *float64 `json:"diagonal_inches,omitempty"`
	RawTenthsInch  uint16   `json:"raw_tenths_inch"`
	RawCentimeters uint8    `json:"raw_centimeters"`
}
type PhysicalSizes struct {
	DP      PhysicalSize `json:"dp"`
	HDMI    PhysicalSize `json:"hdmi"`
	RawMask uint8        `json:"raw_mask"`
}

func ParseDetailedPortInfo(f []byte) (DetailedPortInfo, error) {
	power, err := ParsePortInfo(f)
	if err != nil {
		return DetailedPortInfo{}, err
	}
	usb := func(offset int) USBPort {
		v := f[offset]
		speed := ""
		switch {
		case v&4 != 0:
			speed = "10 Gbps"
		case v&2 != 0:
			speed = "5 Gbps"
		case v&1 != 0:
			speed = "480 Mbps"
		}
		return USBPort{v != 0, v, speed}
	}
	display := func(offset, edid, name int, mask byte) DisplayPort {
		d := DisplayPort{RawRefreshHz: f[offset+4]}
		w, h := binary.BigEndian.Uint16(f[offset:]), binary.BigEndian.Uint16(f[offset+2:])
		if w == 0 || w > 7680 || h == 0 || h > 4320 {
			return d
		}
		d.Connected = true
		d.Width = w
		d.Height = h
		if d.RawRefreshHz > 0 && d.RawRefreshHz <= 240 {
			refresh := d.RawRefreshHz
			d.RefreshHz = &refresh
		}
		d.EDIDPresent = f[23]&mask != 0
		if !d.EDIDPresent {
			return d
		}
		valid := true
		for _, c := range f[edid : edid+3] {
			if c <= 32 || c >= 127 {
				valid = false
			}
		}
		if valid {
			d.Manufacturer = string(f[edid : edid+3])
		}
		d.ProductCode = fmt.Sprintf("%04X", binary.BigEndian.Uint16(f[edid+3:]))
		for _, c := range f[name : name+10] {
			if c == 0 {
				break
			}
			if c >= 32 && c < 127 {
				d.Name += string(c)
			}
		}
		return d
	}
	return DetailedPortInfo{USB: USBPorts{usb(8), usb(9), usb(10), usb(11), usb(12)}, DP: display(13, 24, 34, 1), HDMI: display(18, 29, 44, 2), EthernetLinked: f[3]&1 != 0, PDLinked: power.PDLinked, UFPLinked: power.UFPLinked, HostVendorIndex: binary.BigEndian.Uint16(f[4:]), HostProductIndex: binary.BigEndian.Uint16(f[6:]), RawLinkFlags: f[3], Power: PortPower{UFP: power.UFP, PDIn: power.PDIn, UFPNeg: power.UFPNeg}}, nil
}

// ParsePhysicalSizes decodes diagonal inches, not resolution or refresh rate.
func ParsePhysicalSizes(f []byte) (PhysicalSizes, error) {
	if len(f) != 9 || f[0] != 10 || f[1] != 39 {
		return PhysicalSizes{}, fmt.Errorf("invalid 0A27 response: expected 9 bytes")
	}
	size := func(mask byte, offset int) PhysicalSize {
		s := PhysicalSize{ValidFlag: f[2]&mask != 0}
		if !s.ValidFlag {
			return s
		}
		s.RawTenthsInch = binary.BigEndian.Uint16(f[offset:])
		s.RawCentimeters = f[offset+2]
		if s.RawTenthsInch > 0 && s.RawTenthsInch <= 2000 {
			inches := float64(s.RawTenthsInch) / 10
			s.DiagonalInches = &inches
		}
		return s
	}
	return PhysicalSizes{DP: size(1, 3), HDMI: size(2, 6), RawMask: f[2]}, nil
}
func ScreenSizeRequest() []byte { return []byte{2, 39} }

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
