// Package worker provides interfaces to manage Linux processes
package worker

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/google/uuid"
	"github.com/trivelle/worker/lib/worker/process"
)

// Worker is a Linux process manager
// It keeps a registry of processes that have been requested
// and exposes methods to manage those.
// TODO: add logging to give some observability on worker's actions
type Worker struct {
	processRegistry map[ID]*ProcessHandle
	mu              *sync.RWMutex
}

// ProcessRequest represents a request to execute a Linux process in the worker
type ProcessRequest struct {
	// Command is the command to be executed
	Command string

	// Args is a slice containing the arguments to the command
	Args []string

	// ResourceLimits specifies the resources that the process will have access to
	// These translate to cgroup interface files configuration.
	ResourceLimits ResourceLimits

	// RequestedBy is the user that requested this process request
	RequestedBy string
}

// NewWorker creates an instance of a Worker
func NewWorker(cfg Config) *Worker {
	registry := make(map[ID]*ProcessHandle)
	return &Worker{
		// resourceLimits: cfg.resourceLimitsDefault,
		processRegistry: registry,
		mu:              &sync.RWMutex{},
	}
}

// ID is a unique ID that the worker uses to identify a process
type ID string

// ProcessHandle is a record of a process running in the worker
type ProcessHandle struct {
	process       *process.Process
	outputHandler *OutputHandler
}

// ResourceLimits is a struct that holds requested resource limits
// in a process request. These translate to cgroup interface files.
type ResourceLimits struct {
	MaxMemoryBytes int64
	// ...
}

// StartProcess starts a new process and adds it to the worker
// process registry. It does not wait for the process to terminate.
func (w *Worker) StartProcess(req ProcessRequest) (ID, error) {
	cmd := exec.Command(req.Command, req.Args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	process := process.NewProcess(cmd, req.RequestedBy)

	err = process.Start()
	if err != nil {
		return "", err
	}

	id := ID(uuid.NewString())

	outputHandler, err := NewOutputHandler(stdout, stderr)
	if err != nil {
		return "", err
	}

	processHandle := &ProcessHandle{
		process:       process,
		outputHandler: outputHandler,
	}

	w.addToRegistry(id, processHandle)
	return id, nil
}

// getProcess extracts an *Process instance from the process registry
func (w *Worker) getProcess(processId ID) (*process.Process, error) {
	if handle, ok := w.getFromRegistry(processId); ok {
		return handle.process, nil
	}
	return nil, fmt.Errorf("no process with ID %s", processId)
}

// getOutputHandler extracts an output handler instance from the process registry
func (w *Worker) getOutputHandler(processId ID) (*OutputHandler, error) {
	if handle, ok := w.getFromRegistry(processId); ok {
		return handle.outputHandler, nil
	}
	return nil, fmt.Errorf("no process with ID %s", processId)
}

// addToRegistry adds a new process handle to the registry
func (w *Worker) addToRegistry(processId ID, processHandle *ProcessHandle) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.processRegistry[processId] = processHandle
}

// getFromRegistry retrieves a process handle from the registry
func (w *Worker) getFromRegistry(processId ID) (*ProcessHandle, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	val, ok := w.processRegistry[processId]
	return val, ok
}

// StopProcess stops a process currently managed by the worker
// Returns an error if errors are encountered stopping the process
// or the process does not exist in the worker registry.
func (w *Worker) StopProcess(processId ID) error {
	proc, err := w.getProcess(processId)
	if err != nil {
		return err
	}
	return proc.Stop()
}

// GetProcessStatus gives access to the ProcessStatus struct
// which provides a point in time view of the process status.
func (w *Worker) GetProcessStatus(processId ID) (*process.ProcessStatus, error) {
	proc, err := w.getProcess(processId)
	if err != nil {
		return nil, err
	}
	return proc.GetProcessStatus()
}

// StreamProcessOutput returns an instance of a Streamer that
// can be used to stream the combined stdout and stderr of
// a process managed by the worker
func (w *Worker) StreamProcessOutput(processId ID) (<-chan ProcessOutputEntry, <-chan error, error) {
	outputHandler, err := w.getOutputHandler(processId)
	if err != nil {
		return nil, nil, err
	}
	outChan, errChan := outputHandler.Stream()
	return outChan, errChan, nil
}

// TODO: Add a shutdown method so that clients can gracefully terminate all
// processes.
