package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/trivelle/worker/lib/worker"
)

func main() {
	w := worker.NewWorker(worker.Config{})
	// Start a new process that prints to stdout and stderr every second
	id, err := w.StartProcess(worker.ProcessRequest{
		Command:     "bash",
		Args:        []string{"-c", "for i in {1..5}; do sleep 1; echo \"Hi, $i\"; echo \"error!\" 1>&2; done"},
		RequestedBy: "hashi",
	})
	if err != nil {
		panic(err)
	}

	// Print the process status
	info, err := w.GetProcessStatus(id)
	if err != nil {
		panic(err)
	}
	printInfo(info)

	// Make 1000 goroutines listen to the output
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		i := i
		time.Sleep(time.Millisecond)
		go func() {
			ouputChan, _ := w.StreamProcessOutput(id)
			for line := range ouputChan {
				fmt.Printf("got output in %d: %s\n", i, string(line.Content))
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func printInfo(info *worker.ProcessStatus) {
	fmt.Printf("pid: %v\n", info.PID)
	fmt.Printf("started_by: %s\n", info.StartedBy)

	state := info.State

	fmt.Printf("state: %s\n", state)
	fmt.Printf("started_at: %s\n", info.StartedAt)
	fmt.Printf("finished_at: %s\n", info.FinishedAt)
	fmt.Printf("\n")
}
