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
	masterAddr := flag.String("master-addr", "localhost:9090", "Master address (for worker mode, format: ip:port)")
	workerAddrs := flag.String("worker-addrs", "localhost:9091,localhost:9092,localhost:9093", "Worker addresses for master (comma-separated, format: ip:port)")
	flag.Parse()

	workers := strings.Split(*workerAddrs, ",")

	switch *mode {
	case "master":
		runMasterServer(*masterPort)
	case "worker":
		runWorker(*workerID, *workerPort, *masterAddr)
	case "all":
		runAll(workers, *masterPort)
	case "start-workers":
		startAllWorkers(workers)
	default:
		fmt.Println("Invalid mode. Use: master, worker, all, or start-workers")
	}
}

func runMasterServer(port string) {
	localIP := getOutboundIP()
	master := NewMaster(port, []string{})
	server := NewServer(port, master)

	master.StartCoordinator()

	log.Printf("master started on %s:%s", localIP, port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runWorker(id, port, masterAddress string) {
	worker := NewWorker(id, port, masterAddress, 1000)
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker error: %v", err)
	}
}

func runAll(workers []string, masterPort string) {
	localIP := getOutboundIP()
	master := NewMaster(masterPort, workers)
	server := NewServer(masterPort, master)

	master.StartCoordinator()
	server.StartSSEBroadcaster()

	log.Printf("master started on %s:%s", localIP, masterPort)

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
	masterAddr := "localhost:9090"

	for i, addr := range workers {
		go func(id, a string) {
			worker := NewWorker(id, strings.Split(a, ":")[1], masterAddr, 1000)
			if err := worker.Start(); err != nil {
				log.Printf("Worker error: %v", err)
			}
		}(workerIDs[i], addr)
	}

	time.Sleep(500 * time.Millisecond)
}
