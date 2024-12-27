package main

import (
	"fmt"
	"sync"
)

type Job struct {
	path string
	stop chan struct{}
}

type BuildQueue struct {
	jobs []*Job
	wg   *sync.WaitGroup
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

	newJob := &Job{
		path: job,
		stop: make(chan struct{}),
	}

	for _, j := range q.jobs {
		if j.path == job {
			close(j.stop)
			q.jobs = append(q.jobs, newJob)
			return
		}
	}

	q.jobs = append(q.jobs, newJob)
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
	fmt.Println("[BuildQueue]: emptying queue")
	q.Lock()
	for _, j := range q.jobs {
		close(j.stop)
		q.wg.Done()
	}
	q.jobs = nil
	q.Unlock()

	fmt.Println("[BuildQueue]: queue is empty")
}
