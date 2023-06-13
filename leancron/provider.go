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
	"github.com/draganm/go-lean/common/providers"
	"github.com/draganm/go-lean/gojautils"
	"github.com/draganm/go-lean/leanweb/require"
	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
)

var cronRegexp = regexp.MustCompile(`^.+.cron.js$`)

type GlobalsProviders struct {
	Generic []providers.GenericGlobalsProvider
	Context []providers.ContextGlobalsProvider
}

func Start(
	ctx context.Context,
	src fs.FS,
	root string,
	log logr.Logger,
	loc *time.Location,
	globals map[string]any,
	globalProviders *GlobalsProviders,
) (err error) {

	req, err := require.NewProvider(src, root)
	if err != nil {
		return err
	}

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
		Schedule      string
		AllowParallel bool
		Run           goja.Callable
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

		getCronInfo := func() (*CronInfo, error) {

			vm := goja.New()
			vm.SetFieldNameMapper(gojautils.SmartCapFieldNameMapper)

			for k, v := range globals {
				err = vm.GlobalObject().Set(k, v)
				if err != nil {
					return nil, fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
			allGenericProviders := []providers.GenericGlobalsProvider{req}
			if globalProviders != nil {
				allGenericProviders = append(allGenericProviders, globalProviders.Generic...)
			}

			for _, p := range allGenericProviders {
				vals, err := p(vm)
				if err != nil {
					return nil, fmt.Errorf("could not get values from the global provider: %w", err)
				}
				for k, v := range vals {
					err = vm.GlobalObject().Set(k, v)
					if err != nil {
						return nil, fmt.Errorf("could not set value %s on the global object: %w", k, err)
					}
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
			return info, nil
		}

		ci, err := getCronInfo()
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

			ci, err := getCronInfo()
			if err != nil {
				log.Error(err, "could not get cron info")
				return
			}

			log.Info("cron job started")
			_, err = ci.Run(nil)
			if err != nil {
				log.Error(err, "cron job failed")
			}
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
