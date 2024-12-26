package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

type JobDispatcher struct {
	workers       []*Worker
	workersLock   sync.Mutex
	queue         BuildQueue
	config        *Config
	errors        []error
	wg            *sync.WaitGroup
	stopBroadcast *StopBroadcast
}

func NewJobDispatcher(config *Config, stopBroadcast *StopBroadcast) *JobDispatcher {
	// TODO: add a config for this
	numWorkers := 1

	if config.Parallel {
		numWorkers = runtime.NumCPU()
	}

	workers := make([]*Worker, numWorkers)
	for i := range workers {
		workers[i] = NewWorker(config, i, stopBroadcast)
	}

	return &JobDispatcher{
		workers:       workers,
		queue:         BuildQueue{},
		config:        config,
		errors:        []error{},
		wg:            &sync.WaitGroup{},
		stopBroadcast: stopBroadcast,
	}
}

func (d *JobDispatcher) Start() {
	selectBranches := []reflect.SelectCase{}

	fmt.Printf("JobDispatcher: starting %d workers\n", len(d.workers))

	for i := range d.workers {
		selectBranches = append(selectBranches, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(d.workers[i].Done),
		})
		d.workers[i].Start()
	}

	selectBranches = append(selectBranches, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(d.stopBroadcast.Subscribe()),
	})

	go func() {
		for {
			id, errv, ok := reflect.Select(selectBranches)
			if !ok {
				panic("receive failed")
			}

			// StopBroadcaster fired
			if id == len(selectBranches)-1 {
				fmt.Println("JobDispatcher: stopping")
				d.queue.Clear()
				return
			}

			d.wg.Done()

			if !errv.IsNil() {
				err := errv.Interface().(error)
				d.errors = append(d.errors, err)
			}

			nextJob, ok := d.queue.Dequeue()
			if !ok {
				// The queue is empty
				fmt.Printf("JobDispatcher: worker %02d is done but no jobs are available\n", id)
				continue
			}

			fmt.Printf("JobDispatcher: worker %02d is done, dispatching %s\n", id, nextJob.path)
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
			fmt.Printf("JobDispatcher: worker %02d is already working on %s\n", i, path)
			w.currentJob.stop.Broadcast(struct{}{})
		}
	}

	d.wg.Add(1)

	for i, w := range d.workers {
		if w.currentJob == nil {
			fmt.Printf("JobDispatcher: worker %02d is available, dispatching %s\n", i, path)
			stop := &StopBroadcast{}
			go func(stop *StopBroadcast) {
				<-d.stopBroadcast.Subscribe()
				stop.Broadcast(struct{}{})
				stop.Close()
			}(stop)

			w.AddJob(&Job{
				path: path,
				stop: stop,
			})
			return nil
		}
	}

	fmt.Printf("JobDispatcher: no workers are available, enqueueing %s\n", path)
	d.queue.Enqueue(path)
	return nil
}

func (d *JobDispatcher) Wait() {
	d.wg.Wait()
}
