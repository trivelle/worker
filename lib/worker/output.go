package worker

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// OutputHandler manages output buffering and forwarding to
// concurrent output listeners.
//
// It is initialised with one or more readers which it
// reads to buffer the content and send to listeners
//
// OutputHandler attempts to forward output by chunks of
// length 76.
// TODO: the chunk size should be configurable
//
// Forwarded content for a new listener is always sent from
// the start. Updates are always sent in the order they were
// read. This means output is always in order by reader but
// not necessarily across readers.
//
// TODO: This could possibly be an interface so that the way
// we forward output is configurable. For example, we might
// have an output handler that sends output based on time
// ellapsed.
//
// TODO: At the moment, all OutputHandlers are kept in memory.
// This could be bad if we are running many commands with
// large output (which is not uncommon at all). Consider
// storing into files after a certain period has passed
// or using a database.
type OutputHandler struct {
	listeners []*Listener
	// readers is the readers that the output handler is extracting output from
	// the readers slice is initialised on the constructor and should never
	// change afterwards
	readers []io.Reader
	// combinedBuffer keeps the combined output, this is used to forward to
	// new listeners that came in late
	combinedBuffer []byte
	// messages is a channel where new output is delivered
	messages chan []byte
	// errors is a channel to forward output reading errors to listeners
	errors chan error
	// done is a channel to notify listeners that all output has been read
	done chan struct{}
	mu   *sync.Mutex
}

// ProcessOutputEntry is the struct sent to clients' channels
// streaming output
type ProcessOutputEntry struct {
	Content    []byte
	ReceivedAt time.Time
}

// Listener represents an output listener that the output handler will
// send updates to.
type Listener struct {
	outputChan chan ProcessOutputEntry
	errorChan  chan error
}

// NewOutputHandler returns a new OutputHandler struct from the provided readers.
func NewOutputHandler(rc ...io.Reader) (*OutputHandler, error) {
	if len(rc) == 0 {
		return nil, fmt.Errorf("must provide at least one io.Reader")
	}
	o := &OutputHandler{
		readers:  rc,
		mu:       &sync.Mutex{},
		done:     make(chan struct{}),
		messages: make(chan []byte),
		errors:   make(chan error),
	}
	go o.handleOutput()
	go o.handleBroadcast()
	go o.handleErrors()
	return o, nil
}

// Stream returns a channel to receive output up to the
// present time and stream real time. The channel closes when
// there the process the output is coming from is finished.
// Any errors encountered during output reading are sent through
// the errors channel.
func (o *OutputHandler) Stream() (<-chan ProcessOutputEntry, <-chan error) {
	return o.addListener()
}

// addListener registers a new listener in the output handler and returns
// channels that get all previous output and stream new output.
// The whole operation is behind a lock as this cannot happen at the same
// time as listeners are sent new output or the buffer is being updated.
func (o *OutputHandler) addListener() (chan ProcessOutputEntry, chan error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	// we are going to be sending the existing output entry which is at
	// most one item
	output := make(chan ProcessOutputEntry, 1)
	errChan := make(chan error)

	// catch up on existing output but only send it if there
	// is anything to send.
	if len(o.combinedBuffer) > 0 {
		output <- ProcessOutputEntry{Content: o.combinedBuffer}
	}
	select {
	case <-o.done:
		close(output)
	default:
		o.listeners = append(o.listeners, &Listener{
			outputChan: output,
			errorChan:  errChan,
		})
	}
	return output, errChan
}

func (o *OutputHandler) updateCombinedBuffer(b []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.combinedBuffer = append(o.combinedBuffer, b...)
}

func (o *OutputHandler) getListeners() []*Listener {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.listeners[:len(o.listeners)]
}

// handleBroadcast forwads new messages to the listeners and
// closes listener channels when done signal is received to
// signal end of stream.
func (o *OutputHandler) handleBroadcast() {
L:
	for {
		select {
		case msg := <-o.messages:
			for _, l := range o.getListeners() {
				l.outputChan <- ProcessOutputEntry{Content: msg}
			}
			o.updateCombinedBuffer(msg)
		case <-o.done:
			for _, l := range o.getListeners() {
				close(l.outputChan)
			}
			break L
		}
	}
}

// handleErrors forwards errors to the listeners
func (o *OutputHandler) handleErrors() {
L:
	for {
		select {
		case err := <-o.errors:
			for _, l := range o.getListeners() {
				l.errorChan <- err
			}
			break L
		}
	}
}

// handleOutput manages the goroutines that consume
// the reader to send to buffer and listeners. It
// closes the done channel done.
func (o *OutputHandler) handleOutput() error {
	wg := sync.WaitGroup{}

	readersLen := len(o.readers)
	wg.Add(readersLen)

	// TODO: ideally, we would have a mechanism to halt
	// all goroutines in this OutputHandler if there
	// is an error. Like a quit channel.
	for _, reader := range o.readers {
		go func(r io.Reader) {
			o.bufferAndForwardChunks(r)
			wg.Done()
		}(reader)
	}

	go func() {
		wg.Wait()
		// a closed channel is always ready to receive
		close(o.done)
	}()

	return nil
}

// bufferAndForwardChunks reads the reader by 76 byte chunks
// or it will stop at EOF. It buffers each chunk and forwards
// it to the messages channel.
func (o *OutputHandler) bufferAndForwardChunks(reader io.Reader) {
	buf := make([]byte, 0, 76)
	for {
		n, err := io.ReadFull(reader, buf[:cap(buf)])
		buf = buf[:n]
		if err != nil {
			if err == io.EOF {
				break
			}
			if err != io.ErrUnexpectedEOF {
				o.errors <- fmt.Errorf("failed to read output: %v", err)
				break
			}
		}

		o.messages <- buf
	}
}
