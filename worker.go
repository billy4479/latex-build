package main

import "fmt"

type Worker struct {
	Id            int
	currentJob    *Job
	config        *Config
	newJob        chan *Job
	Done          chan error
	stopBroadcast *StopBroadcast
}

func NewWorker(config *Config, id int, stopBroadcast *StopBroadcast) *Worker {
	return &Worker{
		Id:            id,
		config:        config,
		newJob:        make(chan *Job),
		Done:          make(chan error),
		currentJob:    nil,
		stopBroadcast: stopBroadcast,
	}
}

func (w *Worker) Start() {
	go func() {
		stop := w.stopBroadcast.Subscribe()
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

			case <-stop:
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
