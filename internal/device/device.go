package device

import (
	"context"
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

const (
	AnkerVID = "291a"
	AnkerPID = "010c"
)

type Port struct {
	Path, VID, PID, Manufacturer, SerialNumber string
	IsA210C                                    bool
}

type PublicPort struct {
	Path         string `json:"path"`
	VID          string `json:"vid"`
	PID          string `json:"pid"`
	Manufacturer string `json:"manufacturer"`
	IsA210C      bool   `json:"isA210C"`
}

func (p Port) Public() PublicPort { return PublicPort{p.Path, p.VID, p.PID, p.Manufacturer, p.IsA210C} }

func ListPorts() ([]Port, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]Port, 0, len(ports))
	for _, p := range ports {
		vid := strings.ToLower(p.VID)
		pid := strings.ToLower(p.PID)
		out = append(out, Port{Path: p.Name, VID: vid, PID: pid, Manufacturer: p.Product, SerialNumber: p.SerialNumber, IsA210C: vid == AnkerVID && pid == AnkerPID})
	}
	return out, nil
}
func Find(path string) (Port, error) {
	ps, err := ListPorts()
	if err != nil {
		return Port{}, err
	}
	for _, p := range ps {
		if p.Path == path {
			return p, nil
		}
	}
	return Port{}, fmt.Errorf("serial port not found: %s", path)
}
func Check(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	p, err := Find(path)
	if err != nil {
		return err
	}
	if !p.IsA210C {
		return fmt.Errorf("port %s is not identified as Anker A210C", path)
	}
	return nil
}
