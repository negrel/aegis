package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"text/template"
	"time"

	"github.com/negrel/conc"
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
func StartEnvoy(n conc.Nursery, logger *slog.Logger, xdsPort uint16, adminPort uint16) error {
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
