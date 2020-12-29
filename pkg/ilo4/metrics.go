package ilo4

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"regexp"
	"strings"
)

const (
	LabelLabel    = "label"
	LocationLabel = "location"
	SensorLabel   = "sensor"
	TargetLabel   = "target"
)

var (
	temperatureDesc = prometheus.NewDesc(
		prometheus.BuildFQName("ilo", "server", "temperature_celsius"),
		"iLO sensor temperature in celsius",
		[]string{LabelLabel, LocationLabel, SensorLabel, TargetLabel}, nil,
	)
	labelPrefixRegex = regexp.MustCompile(`^\d+-`)
)

type IloMetrics struct {
	Client *Client
}

func NewIloMetrics(client *Client) *IloMetrics {
	return &IloMetrics{
		Client: client,
	}
}

type temperatureMetric struct {
	Target  string
	Reading TemperatureMeasurement
	Error   error
}

var _ prometheus.Collector = &IloMetrics{}
var _ prometheus.Metric = &temperatureMetric{}

func (t IloMetrics) Describe(descs chan<- *prometheus.Desc) {
	descs <- temperatureDesc
}

func (t IloMetrics) Collect(metrics chan<- prometheus.Metric) {
	// Get data
	h, err := t.Client.GetTemperatures(context.Background())
	if err != nil {
		metrics <- &temperatureMetric{Target: t.Client.URL, Error: err}
		return
	}

	// Publish all readings
	for _, r := range h.Temperature {
		if r.Status == StatusOk {
			metrics <- &temperatureMetric{Target: t.Client.URL, Reading: r}
		}
	}
}

func (m temperatureMetric) Desc() *prometheus.Desc {
	return temperatureDesc
}

func (m temperatureMetric) Write(metric *dto.Metric) error {
	// Failed reading
	if m.Error != nil {
		return m.Error
	}

	// Value
	v := m.Reading.CurrentReading

	// Convert from F to C if needed
	if strings.EqualFold(m.Reading.TempUnit, "fahrenheit") {
		v = (v - 32) / 1.8 // to Celsius
	}

	// Strip prefix for sensor label
	sensor := labelPrefixRegex.ReplaceAllString(m.Reading.Label, "")

	// NOTE labels must be sorted by name
	metric.Label = []*dto.LabelPair{
		{Name: proto.String(LabelLabel), Value: proto.String(m.Reading.Label)},
		{Name: proto.String(LocationLabel), Value: proto.String(m.Reading.Location)},
		{Name: proto.String(SensorLabel), Value: proto.String(sensor)},
		{Name: proto.String(TargetLabel), Value: proto.String(m.Target)},
	}
	metric.Gauge = &dto.Gauge{Value: proto.Float64(v)}
	return nil
}
