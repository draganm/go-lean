package metrics

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
)

type Builder struct {
	files map[string]func() ([]byte, error)
}

func NewBuilder() *Builder {
	return &Builder{
		files: map[string]func() ([]byte, error){},
	}
}

func (b *Builder) Consume(pth string, getContent func() ([]byte, error)) bool {

	if !strings.HasPrefix(pth, "/metrics") {
		return false
	}

	pth = strings.TrimPrefix(pth, "/metrics")

	_, fileName := path.Split(pth)

	handlerSubmatches := metricRegexp.FindStringSubmatch(fileName)

	if len(handlerSubmatches) == 3 {
		b.files[pth] = getContent
		return true
	}

	return false
}

func (b *Builder) Start(ctx context.Context, log logr.Logger, gl globals.Globals) error {

	if len(b.files) == 0 {
		return nil
	}

	c := collector{}

	for pth, getContent := range b.files {

		data, err := getContent()
		if err != nil {
			return fmt.Errorf("could not get content for %s: %w", pth, err)
		}

		_, fileName := path.Split(pth)

		handlerSubmatches := metricRegexp.FindStringSubmatch(fileName)

		if len(handlerSubmatches) == 3 {
			vm := goja.New()
			vm.SetFieldNameMapper(goja.TagFieldNameMapper("lean", false))

			autoWired, err := gl.AutoWire(vm, context.Background())
			if err != nil {
				return fmt.Errorf("could not autowire globals: %w", err)
			}

			for k, v := range autoWired {
				err = vm.GlobalObject().Set(k, v)
				if err != nil {
					return fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
			_, err = vm.RunScript(pth, string(data))
			if err != nil {
				return fmt.Errorf("could not start metric handler %s: %w", pth, err)
			}

			minfo := &metricInfo{}
			err = vm.ExportTo(vm.GlobalObject(), minfo)
			if err != nil {
				return fmt.Errorf("could not export metric %s: %w", pth, err)
			}

			minfo.name = handlerSubmatches[1]
			minfo.metricType = handlerSubmatches[2]
			err = minfo.initialize()
			if err != nil {
				return fmt.Errorf("could not initialize metric %s: %w", pth, err)
			}

			c = append(c, func() prometheus.Metric {
				log := log.WithValues("metric", pth)
				met, err := minfo.collect()
				if err != nil {
					log.Error(err, "could not collect metric")
					return nil
				}
				return met
			})

		}

	}

	prometheus.Register(c)

	go func() {
		<-ctx.Done()
		prometheus.Unregister(c)
	}()

	return nil
}
