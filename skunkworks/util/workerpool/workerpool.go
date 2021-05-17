package workerpool

import (
	"fmt"
)

const (
	defaultMaxQueue = 200
)

// Job represents the job to be run
type Job struct {
	payload interface{}
}

//worker handler
type Handler func(p interface{}) error

// WorkerPool A pool of workers channels that are registered with the workerpool
type WorkerPool struct {
	maxWorkers int
	maxQueue   int
	handler    Handler
	//idle workers for dispatch
	jobPool chan chan Job
	//signal for all worker to stop
	quit chan bool
	//a queue to store all request message payloads to be processed
	jobQueue chan Job
	//remember all workers initialized
	workers []worker
}

func (wp *WorkerPool) Feed(payload interface{}) {
	job := Job{payload: payload}
	wp.jobQueue <- job
}

// NewWorkerPool return a worker pool with maxWorkers workers and maxQueue size job queue.
// If maxQueue == 0, it will set to default value 200.
func NewWorkerPool(maxWorkers, maxQueue int) *WorkerPool {
	if maxQueue == 0 {
		maxQueue = defaultMaxQueue
	}
	return &WorkerPool{
		maxWorkers: maxWorkers,
		maxQueue:   maxQueue,
	}
}

func (wp *WorkerPool) SetHandler(f Handler) *WorkerPool {
	wp.handler = f
	return wp
}

func (wp *WorkerPool) Stop() {
	// stop all workers
	for _, worker := range wp.workers {
		worker.stop()
	}
}

func (wp *WorkerPool) Run() {
	wp.jobPool = make(chan chan Job, wp.maxWorkers)
	wp.jobQueue = make(chan Job, wp.maxQueue)
	wp.quit = make(chan bool)
	wp.workers = make([]worker, wp.maxWorkers)

	// starting n number of workers
	for i := 0; i < wp.maxWorkers; i++ {
		worker := newWorker(wp)
		wp.workers[i] = worker
		worker.start()
	}

	go wp.dispatch()
}

func (wp *WorkerPool) dispatch() {
	fmt.Println("Worker pool dispatcher started...")
	for {
		select {
		case job := <-wp.jobQueue:
			//fmt.Printf("a dispatcher request received")
			// a job request has been received
			go func(job Job) {
				// try to obtain a worker job channel that is available.
				// this will block until a worker is idle
				jobChannel := <-wp.jobPool

				// dispatch the job to the worker job channel
				jobChannel <- job
			}(job)
		}
	}
}
