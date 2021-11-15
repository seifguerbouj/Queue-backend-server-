package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Create 2 tables in the database if they are not yet created
func createTableDone() {
	db := setupDB()
	db.QueryRow("CREATE TABLE IF NOT EXISTS tasks(id SERIAL, taskname varchar(50), done boolean);")
	db.QueryRow("CREATE TABLE IF NOT EXISTS tasksdone(id SERIAL, taskname varchar(50));")

}

var jobs [5]chan Task
var workerPool = make(chan Worker, 10)
var responseChannel = make(chan JsonResponse, 1)
var dbUser *string = new(string)
var dbNameToUse *string = new(string)
var connected *bool = new(bool)
var dbpassword *string = new(string)
var db *sql.DB = new(sql.DB)

func main() {

	// Create backup table for tasks done
	createTableDone()

	// Initialize our queues
	initQueues(jobs[:])

	// Init the mux router
	router := mux.NewRouter()

	// Create our worker pool
	createWorkerPool()

	// Route handles & endpoints

	// Create a task
	router.HandleFunc("/{queue}/tasks", CreateTask).Methods("POST")

	// send task to consumer
	router.HandleFunc("/consumer", ConsumeTask).Methods("POST")

	// Confirm that a task is executed
	router.HandleFunc("/tasks/{id}/done", TaskExecuted).Methods("PUT")

	// serve the app
	fmt.Println("Server at 8000")
	log.Fatal(http.ListenAndServe(":8000", router))
}
