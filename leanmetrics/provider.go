package leanmetrics

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/leanweb/require"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
)

var metricRegexp = regexp.MustCompile(`^(.+).(counter|gauge).metric.js$`)

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
func Start(
	ctx context.Context,
	src fs.FS,
	root string,
	log logr.Logger,
	gl globals.Globals,
) (err error) {
	c := collector{}

	req, err := require.NewProvider(src, root)
	if err != nil {
		return err
	}

	err = fs.WalkDir(src, root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		withoutPrefix := strings.TrimPrefix(pth, root)

		if d.IsDir() {
			return nil
		}

		f, err := src.Open(pth)
		if err != nil {
			return fmt.Errorf("could not open %s: %w", pth, err)
		}

		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", pth, err)
		}

		_, fileName := path.Split(withoutPrefix)

		handlerSubmatches := metricRegexp.FindStringSubmatch(fileName)

		if len(handlerSubmatches) == 3 {
			vm := goja.New()
			vm.SetFieldNameMapper(goja.TagFieldNameMapper("lean", false))

			globs, err := gl.Merge(globals.Globals{"require": req})
			if err != nil {
				return err
			}

			autowired, err := globs.Autowire(vm, context.Background())
			if err != nil {
				return fmt.Errorf("could not autowire globals: %w", err)
			}

			for k, v := range autowired {
				err = vm.GlobalObject().Set(k, v)
				if err != nil {
					return fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
			_, err = vm.RunScript(withoutPrefix, string(data))
			if err != nil {
				return fmt.Errorf("could not start metric handler %s: %w", withoutPrefix, err)
			}

			minfo := &metricInfo{}
			err = vm.ExportTo(vm.GlobalObject(), minfo)
			if err != nil {
				return fmt.Errorf("could not export metric %s: %w", withoutPrefix, err)
			}

			minfo.name = handlerSubmatches[1]
			minfo.metricType = handlerSubmatches[2]
			err = minfo.initialize()
			if err != nil {
				return fmt.Errorf("could not initialize metric %s: %w", withoutPrefix, err)
			}

			c = append(c, func() prometheus.Metric {
				log := log.WithValues("metric", withoutPrefix)
				met, err := minfo.collect()
				if err != nil {
					log.Error(err, "could not collect metric")
					return nil
				}
				return met
			})

		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("could not get metrics: %w", err)
	}

	prometheus.Register(c)

	go func() {
		<-ctx.Done()
		prometheus.Unregister(c)
	}()

	return

}
