package health

import (
	"context"
	"errors"
	"time"
)

// Health check options.
type HealthCheck struct {
	// Check function.
	Test func(context.Context) error
	// Interval at which test function is called.
	Interval time.Duration
	// Test function timeout.
	Timeout time.Duration
	// Number of retry before considering the service unhealthy.
	Retries uint
	// Wait time before first check when starting the process.
	StartPeriod time.Duration
	// Interval between checks until process becomes healthy for the first time.
	StartInterval time.Duration
	// Callback when state change.
	OnChange func(oldState, newState State, err error)
}

// Check starts health check loop. You can stop the loop by canceling the
// provided context.
func Check(ctx context.Context, job HealthCheck) {
	if job.Test == nil {
		panic("health test function is nil")
	}

	var errs error
	state := StateStarting
	setState := func(newState State) {
		if state != newState && job.OnChange != nil {
			job.OnChange(state, newState, errs)
		}
		state = newState
	}

	maxRetry := job.Retries + 1

	// Start health check until healthy.
	{
		sleep(ctx, job.StartPeriod)

		for i := uint(0); i < maxRetry; i++ {
			if ctxDone(ctx) {
				return
			}

			hctx, cancel := context.WithTimeout(ctx, job.Timeout)
			err := job.Test(hctx)
			cancel()

			// Healthy.
			if err == nil {
				errs = nil
				break
			}

			// Handle error.
			errs = errors.Join(errs, err)
			sleep(ctx, job.StartInterval)
		}

		if errs == nil {
			setState(StateHealthy)
		} else {
			if ctxDone(ctx) {
				return
			}
			setState(StateUnhealthy)
		}
	}

	// Monitor health now.
	for {
		errs = nil
		sleep(ctx, job.Interval)

		for i := uint(0); i < maxRetry; i++ {
			if ctxDone(ctx) {
				return
			}

			hctx, cancel := context.WithTimeout(ctx, job.Timeout)
			err := job.Test(hctx)
			cancel()

			// Healthy.
			if err == nil {
				errs = nil
				break
			}

			// Handle error.
			errs = errors.Join(errs, err)
			sleep(ctx, job.Interval)
		}

		if errs == nil {
			setState(StateHealthy)
		} else {
			if ctxDone(ctx) {
				return
			}
			setState(StateUnhealthy)
		}
	}
}

// State enumerates possible state of health checked element.
type State uint8

const (
	StateStarting State = iota
	StateHealthy
	StateUnhealthy
)

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func sleep(ctx context.Context, dur time.Duration) {
	ch := time.After(dur)
	select {
	case <-ch:
	case <-ctx.Done():
	}
}
