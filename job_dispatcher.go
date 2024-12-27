package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

type JobDispatcher struct {
	workers     []*Worker
	workersLock sync.Mutex
	queue       BuildQueue
	config      *Config
	errors      []error
	wg          *sync.WaitGroup
	stopAll     chan struct{}
}

func NewJobDispatcher(config *Config, stopAll chan struct{}) *JobDispatcher {
	numWorkers := config.Parallel

	if numWorkers == 0 {
		numWorkers = runtime.NumCPU()
	}

	wg := &sync.WaitGroup{}
	workers := make([]*Worker, numWorkers)
	for i := range workers {
		workers[i] = NewWorker(config, i, stopAll, wg)
	}

	return &JobDispatcher{
		workers: workers,
		queue: BuildQueue{
			wg: wg,
		},
		config:  config,
		errors:  []error{},
		wg:      wg,
		stopAll: stopAll,
	}
}

func (d *JobDispatcher) Start() {
	selectBranches := []reflect.SelectCase{}

	fmt.Printf("[JobDispatcher]: starting %d workers\n", len(d.workers))

	for i := range d.workers {
		selectBranches = append(selectBranches, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(d.workers[i].Done),
		})
		d.workers[i].Start()
	}

	selectBranches = append(selectBranches, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(d.stopAll),
	})

	go func() {
		for {
			id, errv, ok := reflect.Select(selectBranches)

			// stopAll fired
			if id == len(selectBranches)-1 {
				fmt.Println("[JobDispatcher]: stopping")
				d.queue.Clear()
				return
			}

			if !ok {
				panic("receive failed")
			}

			if !errv.IsNil() {
				err := errv.Interface().(error)
				d.errors = append(d.errors, err)
			}

			nextJob, ok := d.queue.Dequeue()
			if !ok {
				// The queue is empty
				fmt.Printf("[JobDispatcher]: worker %02d is done but no jobs are available\n", id)
				continue
			}

			fmt.Printf("[JobDispatcher]: worker %02d is done, dispatching %s\n", id, nextJob.path)
			d.workers[id].AddJob(nextJob)
		}
	}()
}

func (d *JobDispatcher) AddJob(path string, force bool) error {
	if !force {
		toBuild, err := needsBuild(path, d.config)
		if err != nil {
			return err
		}

		if !toBuild {
			return nil
		}
	}

	d.workersLock.Lock()
	defer d.workersLock.Unlock()

	for i, w := range d.workers {
		if w.currentJob != nil && w.currentJob.path == path {
			fmt.Printf("[JobDispatcher]: worker %02d is already working on %s\n", i, path)
			close(w.currentJob.stop)
		}
	}

	d.wg.Add(1)

	for i, w := range d.workers {
		if w.currentJob == nil {
			fmt.Printf("[JobDispatcher]: worker %02d is available, dispatching %s\n", i, path)
			stop := make(chan struct{})
			go func(stop chan struct{}) {
				<-d.stopAll
				close(stop)
			}(stop)

			w.AddJob(&Job{
				path: path,
				stop: stop,
			})
			return nil
		}
	}

	fmt.Printf("[JobDispatcher]: no workers are available, enqueueing %s\n", path)
	d.queue.Enqueue(path)
	return nil
}

func (d *JobDispatcher) Wait() {
	d.wg.Wait()
}
