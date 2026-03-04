package main

import (
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	mode := flag.String("mode", "all", "Mode: master, worker, or all")
	workerID := flag.String("worker-id", "worker1", "Worker ID (for worker mode)")
	workerPort := flag.String("worker-port", "9091", "Worker port (for worker mode)")
	masterPort := flag.String("master-port", "9090", "Master port (for master/server mode)")
	flag.Parse()

	workers := []string{"9091", "9092", "9093"}

	switch *mode {
	case "master":
		runMasterServer(*masterPort)
	case "worker":
		runWorker(*workerID, *workerPort)
	case "all":
		runAll(workers, *masterPort)
	case "start-workers":
		startAllWorkers(workers)
	default:
		fmt.Println("Invalid mode. Use: master, worker, all, or start-workers")
	}
}

func runMasterServer(port string) {
	master := NewMaster(port, []string{})
	server := NewServer(port, master)

	master.StartCoordinator()

	log.Printf("Starting master server on port %s", port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runWorker(id, port string) {
	worker := NewWorker(id, port, 1000)
	log.Printf("Starting worker %s on port %s", id, port)
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker error: %v", err)
	}
}

func runAll(workers []string, masterPort string) {
	master := NewMaster(masterPort, workers)
	server := NewServer(masterPort, master)

	master.StartCoordinator()
	server.StartSSEBroadcaster()

	go func() {
		log.Printf("Starting server on port %s", masterPort)
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	startAllWorkers(workers)

	select {}
}

func startAllWorkers(workers []string) {
	workerIDs := []string{"worker1", "worker2", "worker3"}

	for i, port := range workers {
		go func(id, p string) {
			worker := NewWorker(id, p, 1000)
			log.Printf("Starting worker %s on port %s", id, p)
			if err := worker.Start(); err != nil {
				log.Printf("Worker error: %v", err)
			}
		}(workerIDs[i], port)
	}

	time.Sleep(500 * time.Millisecond)
}
