package ilo4

import "fmt"

const (
	StatusOk = "OP_STATUS_OK"
)

type HealthTemperature struct {
	HostPwrState string                   `json:"hostpwr_state,omitempty"`
	InPost       int                      `json:"in_post,omitempty"`
	Temperature  []TemperatureMeasurement `json:"temperature,omitempty"`
}

type TemperatureMeasurement struct {
	Label          string  `json:"label,omitempty"`
	XPosition      int     `json:"xposition,omitempty"`
	YPosition      int     `json:"yposition,omitempty"`
	Location       string  `json:"location,omitempty"`
	Status         string  `json:"status,omitempty"`
	CurrentReading float32 `json:"currentreading,omitempty"`
	Caution        float32 `json:"caution,omitempty"`
	Critical       float32 `json:"critical,omitempty"`
	TempUnit       string  `json:"temp_unit,omitempty"`
}

func (m TemperatureMeasurement) String() string {
	if m.Status == StatusOk {
		return fmt.Sprintf("%s %f %s", m.Label, m.CurrentReading, m.TempUnit)
	} else {
		return fmt.Sprintf("%s %s", m.Label, m.Status)
	}
}
