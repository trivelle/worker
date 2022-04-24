package worker

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/trivelle/worker/lib/config"
)

// Worker is a Linux process manager
// It keeps a registry of the execs that have
// been requested
type Worker struct {
	resourceLimits  ResourceLimits
	processRegistry map[ID]*ProcessHandle
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

// NewWorker returns creates an instance of a Worker
func NewWorker(cfg config.Config) *Worker {
	registry := make(map[ID]*ProcessHandle)
	return &Worker{
		// resourceLimits: cfg.resourceLimitsDefault,
		processRegistry: registry,
	}
}

// ID is a unique ID that the worker uses to identify a process
type ID string

type ProcessHandle struct {
	exec          Exec
	outputHandler *OutputHandler
}

type ResourceLimits struct {
	MaxMemoryBytes int64
	// ...
}

type ProcessOutputEntry struct {
	Content    []byte
	ReceivedAt time.Time
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

	process := Process{
		command:   req.Command,
		cmd:       cmd,
		startedBy: req.RequestedBy,
	}

	err = process.Start()
	if err != nil {
		return "", err
	}

	id := ID(uuid.NewString())

	outputHandler, err := NewOutputStreamer(stdout, stderr)
	if err != nil {
		return "", err
	}

	processHandle := &ProcessHandle{
		exec:          &process,
		outputHandler: outputHandler,
	}

	w.processRegistry[id] = processHandle
	return id, nil
}

// getExec extracts an Exec instance from the process registry
func (w *Worker) getExec(processId ID) (Exec, error) {
	if handle, ok := w.processRegistry[processId]; ok {
		return handle.exec, nil
	}
	return nil, fmt.Errorf("no process with ID %s", processId)
}

// StopProcess stops a process currently managed by the worker
// Returns an error if there was any issues stopping the process
// or the process does not exist in the worker registry.
func (w *Worker) StopProcess(processId ID) error {
	exec, err := w.getExec(processId)
	if err != nil {
		return err
	}
	return exec.Stop()
}

// GetProcessStatus gives access to the ProcessInfo interface
// which allows querying for information about the process.
func (w *Worker) GetProcessStatus(processId ID) (*ProcessStatus, error) {
	exec, err := w.getExec(processId)
	if err != nil {
		return nil, err
	}
	return exec.Status()
}

// StreamProcessOutput returns an instance of a Streamer that
// can be used to stream the combined stdout and stderr of
// a process managed by the worker
func (w *Worker) StreamProcessOutput(processId ID) (chan ProcessOutputEntry, chan error) {
	outputChan := w.processRegistry[processId].outputHandler.AddListener()
	return outputChan, nil
}
