package worker

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessStatus(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	err := cmd.Start()
	assert.Nil(t, err, "should not return an error")
	defer cmd.Process.Kill()

	process := newProcess("sleep", cmd, "some_user")

	status, err := process.Status()
	assert.Nil(t, err, "should not return an error")

	assert.Equal(t, "R", status.State)
	assert.Equal(t, cmd.Process.Pid, status.PID)
	assert.Equal(t, "some_user", status.StartedBy)
	// TODO: test the started, finished times
}

func TestProcessStatusError(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	process := newProcess("sleep", cmd, "some_user")

	_, err := process.Status()
	assert.EqualError(t, err, "process not started")
}
