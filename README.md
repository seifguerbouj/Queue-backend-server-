Postgres needs to be installed and running in order to create tables as backup when our server stops.


We have 3 endpoints to our REST API :

1- /{queue}/tasks 
    -> assign a task to a queue and create it in our database

2- /consumer with a query parameter ?queue=
    -> Get a task from a queue and assign it to a worker 

3- /tasks/{id}/done 
    -> Send a confirmation that a task is done and update the database

At first you need to put :
    1- username : for the database
    2- database name : to choose a right database
    3- password : if there is any

The program will create the tables for you if not already created.
The database accepts task name as varchar(50) but we can make it bigger 

For now we have : 
    1- 10 workers running at the same time so we can execute 10 tasks
    2- 5 queues and each queue can hold 10 tasks (going from queue = 1 to queue = 5)
    3- tasks will be put back in queue if not finished after 30 seconds
    4- if queues are all full the new task created will wait until one of the queues have one free spot
    5- if the database have already tasks that are not yet executed, they will be put in a queue (default 1 if empty otherwise one of the others)


There is also another table called tasksdone which will hold all done tasks as a backup or history

To run the program simply write this command in the terminal "go run *.go" 
The minimum go version is set to 1.17 but can be changed in the go.mod file. It should run even if installed version is under 
1.17

It will ask you for :
    1- username 
    2- database name 
    3- password : if there is any

If not successfully connected It will tell you and ask you to enter your credentials again

