package main

import (
	"fmt"
	"sync"
)

type Job struct {
	path string
	// TODO: maybe this one can be just a channel
	stop *StopBroadcast
}

type BuildQueue struct {
	jobs []*Job
	sync.Mutex
}

func (q *BuildQueue) IsEmpty() bool {
	q.Lock()
	l := len(q.jobs)
	q.Unlock()
	return l == 0
}

func (q *BuildQueue) Enqueue(job string) {
	q.Lock()
	defer q.Unlock()

	for _, j := range q.jobs {
		if j.path == job {
			j.stop.Broadcast(struct{}{})
			q.jobs = append(q.jobs, &Job{
				path: job,
				stop: j.stop,
			})
			return
		}
	}

	q.jobs = append(q.jobs, &Job{
		path: job,
		stop: &StopBroadcast{},
	})
}

func (q *BuildQueue) Dequeue() (*Job, bool) {
	q.Lock()
	if len(q.jobs) == 0 {
		q.Unlock()
		return nil, false
	}

	next := q.jobs[0]
	q.jobs = q.jobs[1:]
	q.Unlock()

	return next, true
}

func (q *BuildQueue) Clear() {
	fmt.Println("BuildQueue: emptying queue")
	q.Lock()
	for _, j := range q.jobs {
		j.stop.Broadcast(struct{}{})
		j.stop.Close()
	}
	q.jobs = nil
	q.Unlock()

	fmt.Println("BuildQueue: queue is empty")
}
