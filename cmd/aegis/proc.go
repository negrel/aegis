package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"sync/atomic"
)

// Process define an OS process. It wraps *os.Process to provide a more
// ergonomic and safe interface. For example, you can safely Wait() multiple
// times.
type Process struct {
	os *os.Process

	done chan struct{}
	// State and error are nil until process is done.
	doneState atomic.Pointer[os.ProcessState]
	doneErr   atomic.Pointer[error]

	// Never change.
	stdout *os.File
	stderr *os.File
}

// StartProcess creates a new process for the given command. Created process has
// it's stdout and stderr piped (you must consume Stdout() and Stderr()), it
// inherits environment variables plus the provided variables. This function
// returns an error if it fails to start the process.
func StartProcess(command string, args []string, env []string) (*Process, error) {
	// Prepend command to args.
	args = slices.Clone(args)
	args = slices.Insert(args, 0, command)
	// Resolve command absolute path.
	command, err := exec.LookPath(command)
	if err != nil {
		return nil, err
	}

	// Start process.
	osProc, err := os.StartProcess(command, args, &os.ProcAttr{
		Env: env,
		Files: []*os.File{
			nil,
			os.Stdout,
			os.Stderr,
		},
	})
	if err != nil {
		return nil, err
	}

	proc := &Process{
		os:        osProc,
		done:      make(chan struct{}),
		doneState: atomic.Pointer[os.ProcessState]{},
		doneErr:   atomic.Pointer[error]{},
	}

	go func() {
		state, err := proc.os.Wait()
		proc.doneState.Store(state)
		if err != nil {
			proc.doneErr.Store(&err)
		}
		close(proc.done)
	}()

	return proc, nil
}

// Signal forward signal to process.
func (p *Process) Signal(sig os.Signal) error {
	return p.os.Signal(sig)
}

// GracefulStop starts graceful stop sequence. First, it tries to interrupt the
// process and then it kills once the provided context is done.
func (p *Process) GracefulStop(ctx context.Context) error {
	errCh := make(chan error)
	go func() {
		errCh <- p.os.Signal(os.Interrupt)
	}()

	var err error
	for {
		select {
		case sigErr := <-errCh:
			err = sigErr
		case <-p.done:
			return nil
		case <-ctx.Done():
			return errors.Join(err, p.os.Kill())
		}
	}
}

// Done returns a channel closed once process is done.
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// Wait waits until process has exited and returns a os.ProcessState describing
// its status and an error, if any.
func (p *Process) Wait() (*os.ProcessState, error) {
	<-p.done
	var err error
	perr := p.doneErr.Load()
	if perr != nil {
		err = *perr
	}

	return p.doneState.Load(), err
}

// Pid returns process id.
func (p *Process) Pid() int {
	return p.os.Pid
}
