package worker

import (
	"time"
)

// The following is a very rough sketch of what the library
// might look like.

// Config is the configuration for a Worker instance
type Config struct {
	rootCgroup            string
	resourceLimitsDefault ResourceLimits
	// ...
}

// Worker is a Linux process manager
// After initialising a Worker, a new process can be started by
// calling Execute and calling Start() on the returned Exec instance.
// An Exec currently running in the worker can be retrieved via
// GetExec.
type Worker struct {
	resourceLimits ResourceLimits
	execRegistry   *execRegistry
}

// New returns creates an instance of a Worker
func NewWorker(cfg Config) *Worker {
	return nil
}

// ID is the internal ID that the worker uses to identify an Exec
// Under the hood this might just be a UUID
type ID string

// ExecRegistry holds all active executions in a Worker instance
type execRegistry struct {
	execMap map[ID]*Exec
	// ..
}

type ResourceLimits struct {
	MaxMemoryBytes int64
	// ...
}

// ExecRequest represents a request to execute a Linux process in the worker
type ExecRequest struct {
	// Command is the command executed to be executed together with its arguments
	Command string

	// ResourceLimits specifies the resources that the process will have access to
	// These translate to cgroup interface files configuration.
	ResourceLimits ResourceLimits

	RequestedBy string
}

// Exec represents a process that can be run in the Worker
type Exec interface {
	Command() string
	ProcessId()
	Start() error
	Stop() error
	Info() ExecInfo
	GetOutputStreamer() (Streamer, error)
}

// ExecInfo provides information about the Exec
type ExecInfo interface {
	PID() int
	State() string
	StartedAt() time.Time
	StartedBy() string
	FinishedAt() time.Time
}

// Streamer is an interface used to stream output from a process
// managed by the worker
type Streamer interface {
	// StreamProcessOuput returns a channel to receive ProcessOutputEntry
	// in real time. If any error are encountered during execution the
	// channel will be closed and error will be sent through the error channel.
	StreamProcessOutput() (outputChan <-chan ProcessOutputEntry, errChan <-chan error)
}

type ProcessOutputEntry struct {
	Content    []byte
	ReceivedAt time.Time
}

// Execute instantiates a new Exec and registering it in
// the worker
func (w *Worker) Execute(exec ExecRequest) (Exec, error) {
	return nil, nil
}

// GetExec gets an Exec from the registry in the worker
func (w *Worker) GetExec(processId ID) (Exec, error) {
	return nil, nil
}
