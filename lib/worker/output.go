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
	listeners      []chan ProcessOutputEntry //
	readers        []io.Reader
	combinedBuffer []byte
	messages       chan []byte // messages is a channel where new output is delivered
	done           chan struct{}
	mu             *sync.Mutex
}

// NewOutputHandler returns a new OutputHandler struct from the provided readers
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
func (o *OutputHandler) AddListener() chan ProcessOutputEntry {
	output := make(chan ProcessOutputEntry)
	go func() {
		if o.bufferLen() > 0 {
			output <- ProcessOutputEntry{Content: o.combinedBuffer}
		}
		select {
		case <-o.done:
			close(output)
		default:
			o.addListener(output)
		}
	}()

	return output
}

func (o *OutputHandler) addListener(channel chan ProcessOutputEntry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.listeners = append(o.listeners, channel)
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

func (o *OutputHandler) iterListeners(routine func(chan ProcessOutputEntry)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, listenerChan := range o.listeners {
		routine(listenerChan)
	}
}

func (o *OutputHandler) handleBroadcast() {
L:
	for {
		select {
		case msg := <-o.messages:
			o.iterListeners(func(c chan ProcessOutputEntry) {
				c <- ProcessOutputEntry{Content: msg}
			})
		case <-o.done:
			o.iterListeners(func(c chan ProcessOutputEntry) {
				close(c)
			})
			break L
		}
	}
}

func (o *OutputHandler) handleOutput() error {
	fmt.Println("calling handle output")
	wg := sync.WaitGroup{}

	readersLen := len(o.readers)
	wg.Add(readersLen)

	for _, reader := range o.readers {
		go func(r io.Reader) {
			o.bufferAndForwardLines(r)
			wg.Done()
		}(reader)
	}

	wg.Wait()

	fmt.Println("done with stdout loop, closing listeners")
	close(o.done)
	return nil
}

// bufferAndForwardLines reads the reader line by line or if there
// are no line breaks, it will stop at EOF. It buffers each line
// and forwards it to the messages channel.
func (o *OutputHandler) bufferAndForwardLines(reader io.Reader) {
	r := bufio.NewReader(reader)
	for {
		lineBytes, err := r.ReadBytes('\n')
		if err != nil && err == io.EOF {
			fmt.Println("done reading stdout")
			break
		} else if err != nil {
			fmt.Printf("error reading stdout pipe: %v", err)
		}
		o.updateBuffer(lineBytes)
		fmt.Println("before sending to messages")
		o.messages <- lineBytes
		fmt.Println("after sending to messages")
	}
}
