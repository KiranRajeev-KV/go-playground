package main

import (
	"flag"
	"fmt"
)

func main() {

	mode := flag.String("mode", "", "master or worker")
	clockType := flag.String("clock", "lamport", "lamport or vector")
	port := flag.Int("port", 8080, "worker port")
	id := flag.Int("id", 1, "worker ID for vector clock")
	nodes := flag.Int("nodes", 2, "total number of nodes for vector clock")

	flag.Parse()

	switch *mode {

	case "worker":
		startWorker(*port, *clockType, *id, *nodes)

	case "master":
		workers := []string{
			fmt.Sprintf("http://localhost:%d", *port),
		}
		initClock(*clockType, *nodes)
		startMaster(*clockType, workers)

	default:
		fmt.Println("Usage:")
		fmt.Println("Run worker:")
		fmt.Println("  go run main.go -mode=worker -port=8080 -id=1 -nodes=2 -clock=lamport")
		fmt.Println()
		fmt.Println("Run master:")
		fmt.Println("  go run main.go -mode=master -port=8080 -nodes=2 -clock=lamport")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  Lamport Clock:")
		fmt.Println("    Terminal 1: go run main.go -mode=worker -port=8080 -id=1 -clock=lamport")
		fmt.Println("    Terminal 2: go run main.go -mode=master -port=8080 -clock=lamport")
		fmt.Println()
		fmt.Println("  Vector Clock:")
		fmt.Println("    Terminal 1: go run main.go -mode=worker -port=8080 -id=1 -nodes=2 -clock=vector")
		fmt.Println("    Terminal 2: go run main.go -mode=master -port=8080 -nodes=2 -clock=vector")
	}
}
