package main

import (
	"fmt"
	"sync"
)

type Worker struct {
	Id         int
	currentJob *Job
	config     *Config
	newJob     chan *Job
	Done       chan error
	stop       chan struct{}
	wg         *sync.WaitGroup
}

func NewWorker(config *Config, id int, stop chan struct{}, wg *sync.WaitGroup) *Worker {
	return &Worker{
		Id:         id,
		config:     config,
		newJob:     make(chan *Job),
		Done:       make(chan error, 1),
		currentJob: nil,
		stop:       stop,
		wg:         wg,
	}
}

func (w *Worker) Start() {
	go func() {
		for {
			select {
			case job := <-w.newJob:
				fmt.Printf("Worker %02d: starting job %s\n", w.Id, job.path)

				err := BuildFile(job, w.config, 1)
				fmt.Printf("Worker %02d: job %s completed ", w.Id, job.path)
				if err != nil {
					fmt.Printf("with errors: %v\n", err)
				} else {
					fmt.Println("without errors")
				}

				w.Done <- err
				w.currentJob = nil
				fmt.Printf("Worker %02d calling Done()\n", w.Id)
				w.wg.Done()

			case <-w.stop:
				fmt.Printf("Worker %02d: stopping\n", w.Id)
				return
			}
		}
	}()
}

func (w *Worker) AddJob(job *Job) {
	w.currentJob = job
	w.newJob <- job
}
