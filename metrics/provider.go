package metrics

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/dop251/goja"
	"github.com/prometheus/client_golang/prometheus"
)

var metricRegexp = regexp.MustCompile(`^([^/]+).(counter|gauge).js$`)

type metricProvider func() prometheus.Metric

type collector []metricProvider

func (c collector) Describe(dc chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, dc)
}

func (c collector) Collect(mc chan<- prometheus.Metric) {
	for _, mp := range c {
		if mp != nil {
			mc <- mp()
		}
	}
}

type metricInfo struct {
	metricType     string
	ConstantLabels prometheus.Labels `lean:"constantLabels"`
	Description    string            `lean:"description"`
	Collect        goja.Callable     `lean:"collect"`

	name string
	vt   prometheus.ValueType
	desc *prometheus.Desc
	mu   *sync.Mutex
}

func (m *metricInfo) initialize() error {
	switch m.metricType {
	case "counter":
		m.vt = prometheus.CounterValue
	case "gauge":
		m.vt = prometheus.GaugeValue
	default:
		return fmt.Errorf("unsupported metric type %s", m.metricType)
	}

	if m.ConstantLabels == nil {
		m.ConstantLabels = prometheus.Labels{}
	}

	if m.Description == "" {
		return fmt.Errorf("description is not set")
	}

	if m.Collect == nil {
		return fmt.Errorf("missing function collect()")
	}

	m.desc = prometheus.NewDesc(m.name, m.Description, nil, m.ConstantLabels)
	m.mu = &sync.Mutex{}

	return nil
}

func (m *metricInfo) collect() (prometheus.Metric, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, err := m.Collect(nil)
	if err != nil {
		return nil, fmt.Errorf("could not collect %s: %w", m.name, err)
	}

	return prometheus.MustNewConstMetric(
		m.desc,
		m.vt,
		v.ToFloat(),
	), nil
}

// TODO: figure out how to support variable labels
