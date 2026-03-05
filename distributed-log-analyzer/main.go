package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
)

func main() {
	mode := flag.String("mode", "all", "Mode: master, worker, or all")
	workerID := flag.String("worker-id", "worker1", "Worker ID (for worker mode)")
	workerPort := flag.String("worker-port", "9091", "Worker port (for worker mode)")
	masterPort := flag.String("master-port", "9090", "Master port (for master/server mode)")
	workerAddrs := flag.String("worker-addrs", "localhost:9091,localhost:9092,localhost:9093", "Worker addresses for master (comma-separated, format: ip:port)")
	flag.Parse()

	workers := strings.Split(*workerAddrs, ",")

	switch *mode {
	case "master":
		runMasterServer(*masterPort, workers)
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

func runMasterServer(port string, workers []string) {
	localIP := getOutboundIP()
	master := NewMaster(workers)
	server := NewServer(port, master)

	master.StartCoordinator()

	log.Printf("[INFO] [master] master started on %s:%s", localIP, port)
	log.Printf("[INFO] [master] configured with %d workers: %v", len(workers), workers)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runWorker(id, port string) {
	worker := NewWorker(id, port, 1000)
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker error: %v", err)
	}
}

func runAll(workers []string, masterPort string) {
	localIP := getOutboundIP()
	master := NewMaster(workers)
	server := NewServer(masterPort, master)

	master.StartCoordinator()
	server.StartSSEBroadcaster()

	log.Printf("[INFO] [master] master started on %s:%s", localIP, masterPort)
	log.Printf("[INFO] [master] configured with %d workers: %v", len(workers), workers)

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	startAllWorkers(workers)

	select {}
}

func startAllWorkers(workers []string) {
	workerIDs := []string{"worker1", "worker2", "worker3"}

	for i, addr := range workers {
		go func(id, a string) {
			worker := NewWorker(id, strings.Split(a, ":")[1], 1000)
			if err := worker.Start(); err != nil {
				log.Printf("[ERROR] [main] worker error: %v", err)
			}
		}(workerIDs[i], addr)
	}

	time.Sleep(500 * time.Millisecond)
}
