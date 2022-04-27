package process

import (
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessStatus(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	err := cmd.Start()
	assert.Nil(t, err, "should not return an error")
	defer cmd.Process.Kill()

	process := NewProcess(cmd, "some_user")

	status, err := process.GetProcessStatus()
	assert.Nil(t, err, "should not return an error")

	assert.Equal(t, "R", status.State)
	assert.Equal(t, cmd.Process.Pid, status.PID)
	assert.Equal(t, "some_user", status.StartedBy)
	// TODO: test the started, finished times
}

func TestProcessStatusError(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	process := NewProcess(cmd, "some_user")

	_, err := process.GetProcessStatus()
	assert.EqualError(t, err, "process not started")
}

func TestProcessStartTwiceSequential(t *testing.T) {
	cmd := exec.Command("echo", "hello")

	process := NewProcess(cmd, "some_user")

	err := process.Start()
	assert.Nil(t, err)

	err = process.Start()
	assert.EqualError(t, err, "process already started")
}

// Check that all but one concurrent Start calls succeed
func TestProcessStartMultipleTimesConcurrent(t *testing.T) {
	cmd := exec.Command("echo", "hello")

	process := NewProcess(cmd, "some_user")

	var errorsCountActual int64
	errorsCountExpected := 9
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			err := process.Start()
			if err != nil {
				atomic.AddInt64(&errorsCountActual, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, errorsCountExpected, int(errorsCountActual), "all but one Start calls should have failed")
}

func TestProcessRestartAfterStop(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	process := NewProcess(cmd, "some_user")

	err := process.Start()
	assert.Nil(t, err)

	err = process.Stop()
	assert.Nil(t, err)

	err = process.Start()
	assert.EqualError(t, err, "process already started")
}

func TestProcessStopBeforeStart(t *testing.T) {
	cmd := exec.Command("echo", "hello")

	process := NewProcess(cmd, "some_user")

	err := process.Stop()
	assert.EqualError(t, err, "process not started")
}

func TestProcessStopTwiceSequential(t *testing.T) {
	cmd := exec.Command("echo", "hello")

	process := NewProcess(cmd, "some_user")

	err := process.Start()
	assert.Nil(t, err)

	err = process.Stop()
	assert.Nil(t, err)

	err = process.Stop()
	assert.Nil(t, err)
}

// TODO: this test is potentially flaky if the process gets
// released quicker than expected.
func TestProcessStopMultipleTimesConcurrent(t *testing.T) {
	cmd := exec.Command("sleep", "10")

	process := NewProcess(cmd, "some_user")
	err := process.Start()
	assert.Nil(t, err)

	var errorsCountActual int64
	errorsCountExpected := 0
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			err := process.Stop()
			if err != nil {
				atomic.AddInt64(&errorsCountActual, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, errorsCountExpected, int(errorsCountActual), "no stop calls should fail")
}
