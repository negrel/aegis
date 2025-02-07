package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/negrel/aegis/internal/health"
	"github.com/negrel/sgo"
)

//go:embed envoy.tmpl.yml
var envoyConfigTemplate string

// Envoy wraps underlying Envoy process.
type Envoy struct {
	logger *slog.Logger
	proc   *Process
}

// StartEnvoy starts an Envoy process with the provided configuration and returns
// it. If process failed to start, an error is returned. Logs are forwarded to
// the given logger.
func StartEnvoy(n sgo.Nursery, logger *slog.Logger, xdsPort uint16, adminPort uint16) error {
	// Create config file.
	cfgFile, err := os.CreateTemp(os.TempDir(), "aegis-envoy-config-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for envoy config: %w", err)
	}
	defer cfgFile.Close()

	// Generate config using template.
	tmpl, err := template.New("envoy-config").Parse(envoyConfigTemplate)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(cfgFile, map[string]any{
		"XdsPort":   xdsPort,
		"AdminPort": adminPort,
	})
	if err != nil {
		return fmt.Errorf("failed to create temporary file for envoy config: %w", err)
	}

	// Start Envoy process.
	proc, err := StartProcess("envoy", []string{"-c", cfgFile.Name()}, nil)
	if err != nil {
		return err
	}

	// Remove config file when process is done.
	n.Go(func() error {
		defer os.Remove(cfgFile.Name())
		state, err := proc.Wait()
		if err != nil || state.ExitCode() > 0 {
			panic("envoy exited with a non zero exit code")
		}
		return nil
	})

	// Monitor process health.
	n.Go(func() error {
		health.Check(n, health.HealthCheck{
			Test: func(ctx context.Context) error {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%v", adminPort), nil)
				if err != nil {
					panic(err)
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				if resp.StatusCode != 200 {
					return errors.New(resp.Status)
				}

				return nil
			},
			Interval:      30 * time.Second,
			Timeout:       3 * time.Second,
			Retries:       3,
			StartPeriod:   time.Second,
			StartInterval: time.Second,
			OnChange: func(oldState health.State, newState health.State, err error) {
				if newState == health.StateUnhealthy {
					logger.Error("envoy is not healthy")
				} else if newState == health.StateHealthy {
					logger.Info("envoy is healthy")
				}
			},
		})
		return nil
	})

	// Forward logs.
	n.Go(func() error {
		printLines(proc.Stdout(), os.Stdout, "envoy | ")
		return nil
	})
	n.Go(func() error {
		printLines(proc.Stderr(), os.Stderr, "envoy | ")
		return nil
	})

	// Stop envoy process when nursery is canceled.
	n.Go(func() error {
		<-n.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		logger.Debug("gracefully stopping envoy...")
		err := proc.GracefulStop(ctx)
		if err != nil {
			logger.Error("failed to stop envoy process", slog.Any("error", err))
		} else {
			logger.Info("envoy gracefully stopped")
		}
		return nil
	})

	return nil
}
