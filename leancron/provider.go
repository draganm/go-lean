package leancron

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/leanweb/require"
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

var cronRegexp = regexp.MustCompile(`^.+.cron.js$`)

func Start(
	ctx context.Context,
	src fs.FS,
	root string,
	log logr.Logger,
	loc *time.Location,
	gl globals.Globals,
) (err error) {

	tracer := otel.Tracer("leancron")

	req, err := require.NewProvider(src, root)
	if err != nil {
		return err
	}

	// gl, err = gl.Merge(globals.Globals{"require": req})
	// if err != nil {
	// 	return err
	// }

	scheduler := gocron.NewScheduler(loc)
	go func() {
		<-ctx.Done()
		scheduler.Stop()
	}()

	defer func() {
		if err != nil {
			scheduler.Stop()
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

		if !cronRegexp.MatchString(withoutPrefix) {
			return nil
		}

		getCronInfo := func(ctx context.Context) (*CronInfo, error) {

			vm := goja.New()
			vm.SetFieldNameMapper(goja.TagFieldNameMapper("lean", false))

			require := req(vm)
			vm.GlobalObject().Set("require", require)

			autowired, err := gl.Autowire(ctx, vm)
			if err != nil {
				return nil, fmt.Errorf("could not autowire globals: %w", err)
			}

			for k, v := range autowired {
				err = vm.Set(k, v)
				if err != nil {
					return nil, fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
			_, err = vm.RunScript(withoutPrefix, string(data))
			if err != nil {
				return nil, fmt.Errorf("could not run script %s: %w", withoutPrefix, err)
			}

			info := &CronInfo{}
			err = vm.ExportTo(vm.GlobalObject(), info)
			if err != nil {
				return nil, fmt.Errorf("could not convert value to cron info: %w", err)
			}

			info.durationObserver = executionDuration.WithLabelValues(withoutPrefix)
			info.successCounter = executionSuccessful.WithLabelValues(withoutPrefix)
			info.failureCounter = executionFailed.WithLabelValues(withoutPrefix)
			return info, nil
		}

		ci, err := getCronInfo(context.Background())
		if err != nil {
			return fmt.Errorf("could not get cron info for %s: %w", withoutPrefix, err)
		}

		if ci.Schedule == "" {
			return fmt.Errorf("cron %s does not have `schedule` set", withoutPrefix)
		}

		sch := scheduler
		if !ci.AllowParallel {
			sch = sch.SingletonMode()
		}

		sch.CronWithSeconds(ci.Schedule).DoWithJobDetails(func(job gocron.Job) {
			log := log.WithValues("cronJob", withoutPrefix)

			ctx, span := tracer.Start(job.Context(), fmt.Sprintf("leancron: %s", withoutPrefix))
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

		return nil
	})

	if err != nil {
		return fmt.Errorf("could not get crons: %w", err)
	}

	scheduler.StartAsync()

	return

}
