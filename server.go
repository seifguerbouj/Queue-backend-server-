package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Task struct {
	ID       int    `json:"id"`
	TaskName string `json:"moviename"`
	Done     bool
}

type JsonResponse struct {
	Type    string `json:"type"`
	Data    Task   `json:"data"`
	Message string `json:"message"`
}

// DB set up
func setupDB() *sql.DB {
	var err error
	for !*connected {

		reader := bufio.NewReader(os.Stdin)
		fmt.Println("enter your username for the postgres database")

		*dbUser, _ = reader.ReadString('\n')
		*dbUser = strings.TrimSuffix(*dbUser, "\n")

		fmt.Println("enter your database name for the postgres database")

		*dbNameToUse, _ = reader.ReadString('\n')
		*dbNameToUse = strings.TrimSuffix(*dbNameToUse, "\n")
		fmt.Println("enter your database password for the postgres database")

		*dbpassword, _ = reader.ReadString('\n')

		psqlInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", *dbUser, *dbpassword, *dbNameToUse)

		db, err = sql.Open("postgres", psqlInfo)
		err = db.Ping()
		if err != nil {
			fmt.Println("Your credentials are incorrect please try again!")

		} else {
			*connected = true
			fmt.Println("Successfully connected to", *dbNameToUse, "with user", *dbUser)
		}
	}

	return db
}

func printMessage(message string) {
	fmt.Println("")
	fmt.Println(message)
	fmt.Println("")
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// Create a specific task as recieved from request, add it to the data base
// and then add it to the specific queue if possible, if not then choose an arbitrary queue
func CreateTask(w http.ResponseWriter, r *http.Request) {
	var newTask Task
	vars := mux.Vars(r)
	db := setupDB()
	queueNum, err := strconv.Atoi(vars["queue"])
	queueNum = queueNum - 1

	if err != nil {
		panic(err)
	}
	var response = JsonResponse{}
	err = json.NewDecoder(r.Body).Decode(&newTask)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	taskName := newTask.TaskName
	if taskName == "" {
		response = JsonResponse{Type: "error", Message: "You are missing taskName parameter."}
		responseChannel <- response
	} else {
		var lastInsertID int

		printMessage("Inserting task into DB")

		fmt.Println("Inserting new task with name: " + taskName)

		err := db.QueryRow("INSERT INTO tasks( taskName,done) VALUES($1,$2) returning id;", taskName, false).Scan(&lastInsertID)

		// check errors
		checkErr(err)

		newTask.ID = lastInsertID
		if queueNum < len(jobs) && queueNum > -1 {
			go func() {
				select {
				case jobs[queueNum] <- newTask:
					resp := JsonResponse{Type: "success", Data: newTask, Message: "The task has been inserted successfully in queue number!" + strconv.Itoa(queueNum+1)}
					responseChannel <- resp
				default:
					resp := JsonResponse{Type: "waiting", Data: newTask, Message: "Task created in data base and is waiting to be enqued "}
					responseChannel <- resp
					jobs[queueNum] <- newTask
				}

			}()
		} else {
			responseChannel <- JsonResponse{Type: "FAIL", Data: newTask, Message: "There is no queue number " + strconv.Itoa(queueNum+1) + " Will insert in an arbitrary queue"}
			inserted := false
			queueNum = 0
			for !inserted {
				select {
				case jobs[queueNum] <- newTask:
					inserted = true
				default:
					if queueNum < len(jobs)-1 {
						queueNum++
						jobs[queueNum] <- newTask
						inserted = true
					} else {
						queueNum = 0
						jobs[queueNum] <- newTask
						inserted = true
					}

				}
			}
		}
	}

	json.NewEncoder(w).Encode(<-responseChannel)

}

// Connect to database and put all tasks not done in the queues.
// by default, it will try to populate the first queue but if it is full, it will
// move to the next queue
func getAllTasks(db *sql.DB) {
	var queueNum int = 0
	rows, err := db.Query("SELECT * FROM tasks WHERE done = false")

	// check errors
	checkErr(err)

	// Foreach movie
	for rows.Next() {
		var id int

		var taskName string
		var done bool
		err = rows.Scan(&id, &taskName, &done)

		// check errors
		checkErr(err)
		select {

		case jobs[queueNum] <- Task{ID: id, TaskName: taskName, Done: done}:
			continue
		default:
			if queueNum < len(jobs)-1 {
				queueNum++
				jobs[queueNum] <- Task{ID: id, TaskName: taskName, Done: done}
			} else {
				queueNum = 0
				jobs[queueNum] <- Task{ID: id, TaskName: taskName, Done: done}
			}

		}

	}

}

// Check a specific queue and start a worker and assign a task to it.
func ConsumeTask(w http.ResponseWriter, r *http.Request) {
	var err error
	var queueNum int = 0

	queueNum, err = strconv.Atoi(r.URL.Query().Get("queue"))
	queueNum = queueNum - 1
	if err != nil {
		panic(err)
	}
	if queueNum < len(jobs) && queueNum > -1 {
		select {
		case job := <-jobs[queueNum]:
			select {
			case work := <-workerPool:
				go work.DoTask(job, queueNum)

				w.WriteHeader(http.StatusOK)

				json.NewEncoder(w).Encode(job)
			default:
				response := JsonResponse{Type: "fail", Message: "Workers are all busy"}
				json.NewEncoder(w).Encode(response)
			}

		default:

			json.NewEncoder(w).Encode(JsonResponse{Type: "IDLE", Message: "There are no tasks to process"})
		}
	} else {
		json.NewEncoder(w).Encode(JsonResponse{Type: "FAIL", Message: "Choose a queue number between 1 and " + strconv.Itoa(len(jobs)) + " "})
	}
}

// Get a request from worker and update the data base if task is done.
// Insert the task in another table containing all tasks done as a backup or history
func TaskExecuted(w http.ResponseWriter, r *http.Request) {
	var taskExecuted Task
	vars := mux.Vars(r)
	key := vars["id"]
	db := setupDB()

	err := json.NewDecoder(r.Body).Decode(&taskExecuted)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(key)

	// Check error

	checkErr(err)

	if taskExecuted.ID == id {
		fmt.Println(taskExecuted)
		_, err := db.Exec("UPDATE tasks SET done = $1 WHERE id = $2;", taskExecuted.Done, taskExecuted.ID)
		checkErr(err)
		_, err = db.Exec("INSERT INTO tasksdone(taskName) VALUES($1) returning id;", taskExecuted.TaskName)
		checkErr(err)
	}

}
