package workerpool

import (
	"fmt"
	"github.com/finogeeks/ligase/model"
	"os"
	"strconv"
)

// Job represents the job to be run
type Job struct {
	payload model.Payload
}

//worker handler
type Handler func(p model.Payload) error

type WorkerPool struct {
	// A pool of workers channels that are registered with the workerpool
	maxWorkers int
	handler    Handler
	//idle workers for dispatch
	jobPool chan chan Job
	//signal for all worker to stop
	quit chan bool
	//a queue to store all request message payloads to be processed
	jobQueue chan Job
	//remember all workers initialized
	workers map[int]worker
}

func (wp *WorkerPool) FeedPayload(payload model.Payload) {
	job := Job{payload: payload}
	wp.jobQueue <- job
}

func NewWorkerPool(maxWorkers int) *WorkerPool {
	pool := make(chan chan Job, maxWorkers)
	quit := make(chan bool)

	queue_size, err := strconv.Atoi(os.Getenv("WP_MAX_QUEUE"))
	if err == nil {
		maxQueue = queue_size
	}
	return &WorkerPool{
		jobPool:    pool,
		jobQueue:   make(chan Job, maxQueue),
		maxWorkers: maxWorkers,
		quit:       quit,
		workers:    make(map[int]worker),
	}
}

func (wp *WorkerPool) SetHandler(f Handler) *WorkerPool {
	wp.handler = f
	return wp
}

func (wp *WorkerPool) Stop() {
	// stop all workers
	for i := 0; i < wp.maxWorkers; i++ {
		wp.workers[i].stop()
	}
}

func (wp *WorkerPool) Run() {
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
