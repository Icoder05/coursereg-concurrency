package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-student-service/internal/config"
	"go-student-service/internal/db"
	"go-student-service/internal/seed"
	"go-student-service/internal/server"
)

func main() {
	cfg := config.Load()
	if err := db.Connect(cfg); err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	log.Println("DB connected")

	if cfg.Seed {
		if err := seed.Run(); err != nil {
			log.Fatalf("seeding failed: %v", err)
		}
	}

	srv := server.New(cfg)//intialises server struct that represents server instance

	// graceful shutdown
	go func() {
		log.Printf("starting server on :%s", cfg.HTTPPort) //prints port where server is running
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed  { //(ErrServerClosed is predefined error and srv.ListenAndServe() is starts http server)
			log.Fatalf("server failed: %v", err)
		}
	}()
//By passing a pointer (*http.Request), only the memory address of the request struct is copied 
//waiting for shutdown signal
	stop := make(chan os.Signal, 1)//Creates a channel that will receive OS signals. chan=buffered channel(os.signal is an interface defined from os package) and 1 represnts that this function can catch atleast one signal 
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
//syscall → provides UNIX signals like SIGTERM(UNIX means IPC-interprocess communication (s/w interupt sends asynchronous signals msg to alert imp event to perform a task))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)//(ctx=context, context → used for graceful shutdown timeout.)
	defer cancel()//defer is a unique and powerful keyword in the Go programming language that schedules a function call to be run just before the function containing the defer statement returns
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
//context.WithTimeout creates a new context (ctx) that automatically expires after a specific duration 
//(5*time.Second). This is used to define a maximum time limit for the server to shut down cleanly. 
//cancel is a function provided to immediately stop waiting if the shutdown happens faster than the timeout