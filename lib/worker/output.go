package worker

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

// OutputHandler manages output buffering and forwarding to
// concurrent output listeners.
//
// It is initialised with one or more readers and which it
// buffers content from to then forward it to the listeners.
//
// OutputHandler attempts to forward output line by line but
// if it doesn't find any line breaks, it will do so when an
// EOF is encountered.
//
// Forwarded content for a new listener is always sent from
// the start. Updates are always sent in the order they were
// read. This means output is always in order by reader but
// not across readers.
//
// TODO: This could possibly be an interface so that the way
// we forward output is configurable. For example, we might
// have an output handler that sends output based on time
// ellapsed instead of line breaks.
//
// TODO: At the moment, all buffers are kept in memory.
// This could be bad if we are running many commands with
// large output (which is not uncommon at all). Consider
// storing into files after a certain period has passed
// or using a database.
type OutputHandler struct {
	listeners      []*Listener
	readers        []io.Reader
	combinedBuffer []byte
	messages       chan []byte // messages is a channel where new output is delivered
	done           chan struct{}
	mu             *sync.Mutex
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
		return nil, fmt.Errorf("must provide at least one io.ReadCloser")
	}
	o := &OutputHandler{
		readers:  rc,
		mu:       &sync.Mutex{},
		done:     make(chan struct{}),
		messages: make(chan []byte),
	}
	go o.handleOutput()
	go o.handleBroadcast()
	return o, nil
}

// AddListener returns a channel to receive output up to the
// present time and stream real time. The channel closes when
// there the process the output is coming from is finished.
func (o *OutputHandler) AddListener() (chan ProcessOutputEntry, chan error) {
	output := make(chan ProcessOutputEntry)
	errChan := make(chan error)

	go func() {
		if o.bufferLen() > 0 {
			output <- ProcessOutputEntry{Content: o.combinedBuffer}
		}
		select {
		case <-o.done:
			close(output)
		default:
			o.addListener(&Listener{
				outputChan: output,
				errorChan:  errChan,
			})
		}
	}()

	return output, errChan
}

func (o *OutputHandler) addListener(newListener *Listener) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.listeners = append(o.listeners, newListener)
}

func (o *OutputHandler) updateBuffer(b []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.combinedBuffer = append(o.combinedBuffer, b...)
}

func (o *OutputHandler) bufferLen() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.combinedBuffer)
}

func (o *OutputHandler) iterListeners(routine func(*Listener)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, l := range o.listeners {
		routine(l)
	}
}

// handleBroadcast forwads new messages to the listeners and
// closes listener channels when done signal is received to
// signal end of stream.
func (o *OutputHandler) handleBroadcast() {
L:
	for {
		select {
		case msg := <-o.messages:
			o.iterListeners(func(l *Listener) {
				l.outputChan <- ProcessOutputEntry{Content: msg}
			})
		case <-o.done:
			o.iterListeners(func(l *Listener) {
				close(l.outputChan)
			})
			break L
		}
	}
}

// handleOutput manages the goroutines that consume
// the reader to send to buffer and listeners. It
// closes the done channel done.
func (o *OutputHandler) handleOutput() error {
	fmt.Println("calling handle output")
	wg := sync.WaitGroup{}

	readersLen := len(o.readers)
	wg.Add(readersLen)

	// TODO: we are not really doing anything with the error channel
	// at the moment. Ideally, we would use the error channel to
	// trigger listener channel close and quit goroutines.
	errChan := make(chan error)

	for _, reader := range o.readers {
		go func(r io.Reader) {
			o.bufferAndForwardLines(r, errChan)
			wg.Done()
		}(reader)
	}

	go func() {
		wg.Wait()
		close(o.done)
		fmt.Println("done with stdout loop, closing listeners")
	}()

	return nil
}

// bufferAndForwardLines reads the reader line by line or if there
// are no line breaks, it will stop at EOF. It buffers each line
// and forwards it to the messages channel.
func (o *OutputHandler) bufferAndForwardLines(reader io.Reader, errorChan chan<- error) {
	r := bufio.NewReader(reader)
	for {
		lineBytes, err := r.ReadBytes('\n')
		if err != nil && err == io.EOF {
			fmt.Println("done reading reader")
			break
		} else if err != nil {
			fmt.Printf("error reading reader: %v", err)
			errorChan <- err
			break
		}
		o.updateBuffer(lineBytes)
		o.messages <- lineBytes
	}
}
