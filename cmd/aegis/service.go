package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/negrel/aegis/internal/xnet"
	"github.com/negrel/conc"
)

// StartService starts a service process.
func StartService(n conc.Nursery, logger *slog.Logger, service string) (uint16, error) {
	// Determinate service TCP port.
	lis, tcpPort, err := xnet.RandomListener("tcp")
	if err != nil {
		return 0, fmt.Errorf("failed to listen on random TCP port: %w", err)
	}
	lis.Close()

	getEnv := func(key string) string {
		if key == "PORT" {
			return strconv.Itoa(int(tcpPort))
		} else {
			return os.Getenv(key)
		}
	}

	// Parse service command and substitute environment variables.
	args := strings.Split(service, " ")
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%v", tcpPort))
	for i, arg := range args {
		if strings.Contains(arg, "=") {
			env = append(env, os.Expand(arg, getEnv))
		} else {
			args = args[i:]
			break
		}
	}
	for i, arg := range args {
		args[i] = os.Expand(arg, getEnv)
	}

	// Start service process.
	proc, err := StartProcess(args[0], args[1:], env)
	if err != nil {
		return 0, fmt.Errorf("failed to start service process %v: %w", args, err)
	}

	// Stop process when nursery is canceled.
	n.Go(func() error {
		<-n.Done()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		logger.Debug("gracefully stopping service...")
		err := proc.GracefulStop(ctx)
		if err != nil {
			logger.Error("failed to stop service process", slog.Any("error", err))
		} else {
			logger.Info("service gracefully stopped")
		}
		return nil
	})

	return tcpPort, nil
}
