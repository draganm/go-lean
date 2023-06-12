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
	"github.com/draganm/go-lean/gojautils"
	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
)

var cronRegexp = regexp.MustCompile(`^.+.cron.js$`)

func Start(
	ctx context.Context,
	src fs.FS,
	root string,
	log logr.Logger,
	loc *time.Location,
	globals map[string]any,
) (err error) {

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

		vm := goja.New()
		vm.SetFieldNameMapper(gojautils.SmartCapFieldNameMapper)

		for k, v := range globals {
			err = vm.GlobalObject().Set(k, v)
			if err != nil {
				return fmt.Errorf("could not set global %s for cron %s: %w", k, withoutPrefix, err)
			}
		}

		val, err := vm.RunScript(withoutPrefix, string(data))
		if err != nil {
			return fmt.Errorf("could not run script %s: %w", withoutPrefix, err)
		}

		info := &CronInfo{}
		err = vm.ExportTo(val, info)
		if err != nil {
			return fmt.Errorf("could not convert value to cron info: %w", err)
		}

		sch := scheduler
		if !info.AllowParallel {
			sch = sch.SingletonMode()
		}

		sch.CronWithSeconds(info.Schedule).DoWithJobDetails(func(job gocron.Job) {
			log := log.WithValues("cronJob", withoutPrefix)
			log.Info("cron job started")
			_, err := info.Run(nil)
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
