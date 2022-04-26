package worker_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trivelle/worker/lib/worker"
)

func TestOutputHandlerEmptyReader(t *testing.T) {
	var stdoutReader bytes.Buffer

	outputHandler, err := worker.NewOutputHandler(&stdoutReader)
	assert.Nil(t, err)
	outputChan, _ := outputHandler.Stream()

	expectedOutputLines := []string{}

	var outputLines []string
	for out := range outputChan {
		outputLines = append(outputLines, string(out.Content))
	}

	assert.ElementsMatch(t, outputLines, expectedOutputLines)
}

func TestOutputHandlerSuccess(t *testing.T) {
	var stdoutReader bytes.Buffer
	var stdErrReader bytes.Buffer

	_, err := stdoutReader.WriteString("some output line 1\nsome output line 2\nsome output line 3")
	assert.Nil(t, err, "error setting up reader")

	_, err = stdErrReader.WriteString("some error")
	assert.Nil(t, err, "error setting up reader")

	outputHandler, err := worker.NewOutputHandler(&stdoutReader, &stdErrReader)
	assert.Nil(t, err)
	outputChan, _ := outputHandler.Stream()

	expectedOutputLines := []string{"some output line 1\nsome output line 2\nsome output line 3", "some error"}

	var outputLines []string
	for out := range outputChan {
		outputLines = append(outputLines, string(out.Content))
	}

	assert.ElementsMatch(t, outputLines, expectedOutputLines)
}

// errorReader is a fake reader that throws errors for testing
type errorReader struct {
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("errorReader returns errors")
}

func TestOutputHandlerError(t *testing.T) {
	var stdoutReader bytes.Buffer
	errorReader := &errorReader{}

	_, err := stdoutReader.WriteString("some output line 1\nsome output line 2\nsome output line 3")
	assert.Nil(t, err, "error setting up reader")

	outputHandler, err := worker.NewOutputHandler(&stdoutReader, errorReader)
	outputChan, errChan := outputHandler.Stream()

	for range outputChan {
		// do nothing, just consume output
	}

	err = <-errChan
	assert.EqualError(t, err, "failed to read output: errorReader returns errors")
}
