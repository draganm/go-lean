package cron

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/common/goja/fieldmapper"
	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
)

var (
	executionDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name: "leancron_execution_duration",
		Help: "Execution duration of a cron job",
	}, []string{"cron"})

	executionSuccessful = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "leancron_execution_successful_count",
		Help: "Number of successful execution of a cron job",
	}, []string{"cron"})

	executionFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "leancron_execution_failed_count",
		Help: "Number of failed execution of a cron job",
	}, []string{"cron"})
)

var cronRegexp = regexp.MustCompile(`^/cron/.+.js$`)

var tracer = otel.Tracer("leancron")

//

type Builder struct {
	files map[string]func() ([]byte, error)
}

func NewBuilder() *Builder {
	return &Builder{
		files: map[string]func() ([]byte, error){},
	}
}

func (b *Builder) Consume(pth string, getContent func() ([]byte, error)) bool {

	if cronRegexp.MatchString(pth) {
		b.files[pth] = getContent
		return true
	}

	return false
}

func (b *Builder) Start(ctx context.Context, log logr.Logger, gl globals.Globals) (err error) {

	if len(b.files) == 0 {
		log.Info("no crons found")
		return nil
	}

	scheduler := gocron.NewScheduler(time.Local)

	defer func() {
		if err != nil {
			scheduler.Stop()
		} else {
			go func() {
				<-ctx.Done()
				scheduler.Stop()
			}()
		}
	}()

	type CronInfo struct {
		Schedule         string        `lean:"schedule"`
		AllowParallel    bool          `lean:"allowParallel"`
		Run              goja.Callable `lean:"run"`
		durationObserver prometheus.Observer
		successCounter   prometheus.Counter
		failureCounter   prometheus.Counter
	}

	for pth, getData := range b.files {
		data, err := getData()
		if err != nil {
			return fmt.Errorf("could not get data for %s: %w", pth, err)
		}

		getCronInfo := func(ctx context.Context) (*CronInfo, error) {

			vm := goja.New()
			vm.SetFieldNameMapper(fieldmapper.FallbackFieldMapper{})

			autoWired, err := gl.AutoWire(ctx, vm)
			if err != nil {
				return nil, fmt.Errorf("could not autowire globals: %w", err)
			}

			for k, v := range autoWired {
				err = vm.Set(k, v)
				if err != nil {
					return nil, fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
			_, err = vm.RunScript(pth, string(data))
			if err != nil {
				return nil, fmt.Errorf("could not run script %s: %w", pth, err)
			}

			info := &CronInfo{}
			err = vm.ExportTo(vm.GlobalObject(), info)
			if err != nil {
				return nil, fmt.Errorf("could not convert value to cron info: %w", err)
			}

			info.durationObserver = executionDuration.WithLabelValues(pth)
			info.successCounter = executionSuccessful.WithLabelValues(pth)
			info.failureCounter = executionFailed.WithLabelValues(pth)
			return info, nil
		}

		ci, err := getCronInfo(context.Background())
		if err != nil {
			return fmt.Errorf("could not get cron info for %s: %w", pth, err)
		}

		if ci.Schedule == "" {
			return fmt.Errorf("cron %s does not have `schedule` set", pth)
		}

		sch := scheduler
		if !ci.AllowParallel {
			sch = sch.SingletonMode()
		}

		sch.CronWithSeconds(ci.Schedule).DoWithJobDetails(func(job gocron.Job) {
			log := log.WithValues("cronJob", pth)

			ctx, span := tracer.Start(job.Context(), fmt.Sprintf("leancron: %s", pth))
			defer span.End()

			ci, err := getCronInfo(ctx)
			if err != nil {
				log.Error(err, "could not get cron info")
				return
			}

			startTime := time.Now()
			log.Info("cron job started")
			_, err = ci.Run(nil)
			ci.durationObserver.Observe(time.Since(startTime).Seconds())
			if err != nil {
				ci.failureCounter.Inc()
				log.Error(err, "cron job failed")
				span.RecordError(err)
				return
			}
			ci.successCounter.Inc()
			log.Info("cron job successful")
		})

	}

	scheduler.StartAsync()

	return nil

}
