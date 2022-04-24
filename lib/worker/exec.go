package worker

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"
)

// Exec represents something to be executed in the Worker
type Exec interface {
	Command() string
	Start() error
	Stop() error
	PID() int
	Status() (*ProcessStatus, error)
}

type Process struct {
	command    string
	cmd        *exec.Cmd
	startedBy  string
	startedAt  time.Time
	finishedAt time.Time
}

// ProcessStatus represents a snapshot of a process status
type ProcessStatus struct {
	PID        int
	StartedBy  string
	State      string
	StartedAt  time.Time
	FinishedAt time.Time
}

func (p *Process) Start() error {
	p.startedAt = time.Now()
	fmt.Println(p.startedAt)
	err := p.cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func (p *Process) Stop() error {
	if p.cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	return p.cmd.Process.Kill()
}

func (p *Process) Command() string {
	return p.command
}

func (p *Process) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// retrieveProcessState retrieves the Linux process state
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

func (p *Process) Status() (*ProcessStatus, error) {
	pid := p.PID()
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
