package worker

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

type OutputHandler struct {
	listeners      []chan ProcessOutputEntry
	readClosers    []io.Reader
	combinedBuffer []byte
	messages       chan []byte // messages is a channel where new output is delivered
	done           chan struct{}
	mu             *sync.Mutex
}

// NewOutputStreamer returns the OutputHandler struct to stream output
func NewOutputStreamer(rc ...io.Reader) (*OutputHandler, error) {
	if len(rc) == 0 {
		return nil, fmt.Errorf("must provide at least one io.ReadCloser")
	}
	o := &OutputHandler{
		readClosers: rc,
		mu:          &sync.Mutex{},
		done:        make(chan struct{}),
		messages:    make(chan []byte),
	}
	go o.handleOutput()
	go o.handleBroadcast()
	return o, nil
}

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

	wg.Add(1)
	go func() {
		o.readLines(o.readClosers[0])
		wg.Done()
	}()
	wg.Wait()

	fmt.Println("done with stdout loop, closing listeners")
	close(o.done)
	return nil
}

func (o *OutputHandler) readLines(readCloser io.Reader) {
	reader := bufio.NewReader(readCloser)
	for {
		lineBytes, err := reader.ReadBytes('\n')
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
