package workerpool

import (
	"fmt"
)

// Worker represents the worker that executes the job
type worker struct {
	workerPool *WorkerPool
	//as a lock for this job
	jobChannel chan Job
	quit       chan bool
	id         int
}

func newWorker(workerPool *WorkerPool) worker {
	return worker{
		workerPool: workerPool,
		jobChannel: make(chan Job),
		quit:       make(chan bool),
	}
}

// Start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (w worker) start() {
	go func() {
		for {
			// register the current worker into the worker queue.
			w.workerPool.jobPool <- w.jobChannel

			select {
			case job := <-w.jobChannel:
				// we have received a work request.
				if err := w.workerPool.handler(job.payload); err != nil {
					fmt.Printf("Error handling: %s", err.Error())
				}

			case <-w.quit:
				// we have received a signal to stop
				return
			}
		}
	}()
}

// Stop signals the worker to stop listening for work requests.
func (w worker) stop() {
	go func() {
		w.quit <- true
	}()
}
