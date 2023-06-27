package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/leancron"
	"github.com/draganm/go-lean/leanhttp"
	"github.com/draganm/go-lean/leanmetrics"
	"github.com/draganm/go-lean/leanweb"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

//go:embed lean
var leanfs embed.FS

func main() {
	logger, _ := zap.Config{
		Encoding:    "json",
		Level:       zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:   "message",
			LevelKey:     "level",
			EncodeLevel:  zapcore.CapitalLevelEncoder,
			TimeKey:      "time",
			EncodeTime:   zapcore.ISO8601TimeEncoder,
			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}.Build()
	defer logger.Sync()

	app := &cli.App{
		Name: "template",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Value:   ":5001",
				EnvVars: []string{"ADDR"},
			},

			&cli.StringFlag{
				Name:    "metrics-addr",
				Value:   ":3001",
				EnvVars: []string{"METRICS_ADDR"},
			},
		},
		Action: func(c *cli.Context) (err error) {
			log := zapr.NewLogger(logger)

			defer func() {
				if err != nil {
					log.Error(err, "error ocurred")
				}
			}()

			eg, ctx := errgroup.WithContext(c.Context)

			httpProvider := leanhttp.NewProvider(http.DefaultClient)

			// start web handler
			webhandler, err := leanweb.New(leanfs, "lean/web", log, globals.Globals{"http": httpProvider})
			if err != nil {
				return fmt.Errorf("could not start lean web: %w", err)
			}
			eg.Go(runHttp(ctx, log, c.String("addr"), "web", webhandler))

			// start lean cron
			err = leancron.Start(ctx, leanfs, "lean/cron", log, time.Local, globals.Globals{"http": httpProvider})
			if err != nil {
				return fmt.Errorf("could not start lean cron: %w", err)
			}

			// start lean metrics
			err = leanmetrics.Start(ctx, leanfs, "lean/metrics", log, map[string]any{})
			if err != nil {
				return fmt.Errorf("could not start lean metrics: %w", err)
			}

			metricsMux := &http.ServeMux{}
			metricsMux.Handle("/metrics", promhttp.Handler())
			eg.Go(runHttp(ctx, log, c.String("metrics-addr"), "metrics", metricsMux))

			eg.Go(func() error {

				sigs := make(chan os.Signal, 1)
				signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case sig := <-sigs:
					log.Info("signal received, terminating", "sig", sig)
					return fmt.Errorf("signal %s received", sig.String())
				}

			})

			return eg.Wait()
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		os.Exit(1)
	}

}

func runHttp(ctx context.Context, log logr.Logger, addr, name string, handler http.Handler) func() error {

	return func() error {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("could not listen for %s requests: %w", name, err)

		}

		s := &http.Server{
			Handler: handler,
		}

		go func() {
			<-ctx.Done()
			shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			log.Info(fmt.Sprintf("graceful shutdown of the %s server", name))
			err := s.Shutdown(shutdownContext)
			if errors.Is(err, context.DeadlineExceeded) {
				log.Info(fmt.Sprintf("%s server did not shut down gracefully, forcing close", name))
				s.Close()
			}
		}()

		log.Info(fmt.Sprintf("%s server started", name), "addr", l.Addr().String())
		return s.Serve(l)
	}
}
