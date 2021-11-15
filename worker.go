package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Consumer interface {
	DoTask(task Task)
	TaskDone()
}

type Worker struct {
	ID  int
	job Task
}

// Recieve a task, do some work on it and then mark it as done.
// if task is done under 30 seconds then call TaskDone
// else return the task to its queue and worker to worker queue
func (c Worker) DoTask(job Task, queueNum int) {
	start := time.Now()
	fmt.Println("Worker ", c.ID, " is working on job with id ", job.ID, "and name", job.TaskName)
	duration := time.Duration(rand.Intn(1e3)) * time.Millisecond
	time.Sleep(duration)
	time.Sleep(time.Second * 10)

	if int(time.Since(start).Seconds()) < 30 {
		fmt.Println("Time elapsed :", int(time.Since(start).Seconds()), "seconds")
		fmt.Println("Worker ", c.ID, " completed work on job with name ", job.TaskName, " within ", duration)
		c.job = job
		c.job.Done = true
		c.TaskDone()
	} else {
		inserted := false
		for !inserted {
			select {
			case jobs[queueNum] <- job:
				fmt.Println(" Worker was not able to finish job under 30 s job is being inserted in queue", queueNum+1)
				inserted = true
				workerPool <- c
			default:
				if queueNum < len(jobs)-1 {
					queueNum++
					fmt.Println(" Worker was not able to finish job under 30 s job is being inserted in queue", queueNum+1)
					jobs[queueNum] <- job
					workerPool <- c
					inserted = true
				} else {
					queueNum = 0
					fmt.Println(" Worker was not able to finish job under 30 s job is being inserted in queue", queueNum+1)
					jobs[queueNum] <- job
					workerPool <- c
					inserted = true
				}
			}
		}
	}

}

// Send a PUT request having the task ID in the URL to tell the REST API
// that that specific task is done.
// Return worker to worker pool
func (c Worker) TaskDone() {
	client := &http.Client{}
	if c.job.ID != 0 {
		fmt.Println("worker ", c.ID, " completed task with id ", c.job.ID, " and name ", c.job.TaskName)

		postBody, _ := json.Marshal(c.job)
		responseBody := bytes.NewBuffer(postBody)

		req, _ := http.NewRequest(http.MethodPut, "http://localhost:8000/tasks/"+strconv.Itoa(c.job.ID)+"/done", responseBody)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		client.Do(req)

		c.job = Task{}
		workerPool <- c
	}

}

// Create worker pool and populate it with workers.
// Each worker have a specific ID.
func createWorkerPool() {
	for i := 0; i < 10; i++ {
		workerPool <- Worker{ID: i}
	}
	db := setupDB()
	go getAllTasks(db)
}

// Create queues as a table of channels.
// Each queue is a buffered channel of capacity 10.
func initQueues(jobs []chan Task) {
	for i := range jobs {
		jobs[i] = make(chan Task, 10)
	}
}
