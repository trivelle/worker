package process

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Process is a Process executed or to be executed by the worker
// TODO: Find a better name for this as the word "Process" implies
// that this it is already running.
type Process struct {
	command    string
	cmd        *exec.Cmd
	startedBy  string
	startedAt  time.Time
	finishedAt time.Time // TODO: implement finished at
	mu         sync.Mutex
}

func NewProcess(command string, cmd *exec.Cmd, startedBy string) *Process {
	return &Process{
		command:   command,
		cmd:       cmd,
		startedBy: startedBy,
		mu:        sync.Mutex{},
	}
}

// ProcessStatus represents a snapshot of a process status
type ProcessStatus struct {
	PID        int
	StartedBy  string
	State      string
	StartedAt  time.Time
	FinishedAt time.Time // TODO: implement finished at
}

// Start starts a process. It can only be done once otherwise
// it will return an error
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd.Process != nil {
		return fmt.Errorf("process already started")
	}
	p.startedAt = time.Now()
	err := p.cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

// Stop stops a process. It should only be done once
// Subsequent calls to Stop would either be a noop
// or return "os: process already released" error
// coming from p.cmd.Process.Kill().
// Returns an error if the process has not been started
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	return p.cmd.Process.Kill()
}

// Command returns the command of the process
func (p *Process) Command() string {
	return p.command
}

func (p *Process) getPid() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// GetProcessStatus returns ProcessStatus corresponding to the
// process as a point in time status of the process.
func (p *Process) GetProcessStatus() (*ProcessStatus, error) {
	if p.cmd.Process == nil {
		return nil, fmt.Errorf("process not started")
	}
	pid := p.getPid()
	state, err := retrieveProcessState(pid)
	if err != nil {
		return nil, err
	}

	return &ProcessStatus{
		PID:        pid,
		StartedBy:  p.startedBy,
		State:      state,
		StartedAt:  p.startedAt,
		FinishedAt: p.finishedAt,
	}, nil
}

// WithStatus calls f ensuring that the ProcessStatus is not changed
// by the worker until f finishes. This does not guarantee that the
// OS will not change the status of the process. For example, memory
// exhaustion issues can kill the process as the ProcessStatus struct
// is read.
func (p *Process) WithStatus(f func(*ProcessStatus) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	processStatus, err := p.GetProcessStatus()
	if err != nil {
		return err
	}
	return f(processStatus)
}

// retrieveProcessState retrieves the Linux process state i.e.
// one of R, D, S, T or Z.
// TODO: this could belong to its own internal os package but for now I think
// it is fine here.
func retrieveProcessState(pid int) (string, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	dataBytes, err := ioutil.ReadFile(statPath)
	if err != nil {
		return "", nil
	}

	// Move past the image name as process state is right after
	data := string(dataBytes)
	binStart := strings.IndexRune(data, '(') + 1
	binEnd := strings.IndexRune(data[binStart:], ')')
	data = data[binStart+binEnd+2:]

	splittedData := strings.Split(data, " ")
	if len(splittedData) < 1 {
		return "", fmt.Errorf("malformed proc stat data")
	}
	state := splittedData[0]

	return state, nil
}
