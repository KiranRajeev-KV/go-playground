package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

var (
	lamportTime int
	mu          sync.Mutex
	vectorTime  []int
	workerID    int
)

func updateLamportTime(recv int) int {
	mu.Lock()
	defer mu.Unlock()

	if recv > lamportTime {
		lamportTime = recv
	}

	lamportTime++

	return lamportTime
}

func updateVectorTime(recv []int) []int {
	mu.Lock()
	defer mu.Unlock()

	for i := range len(recv) {
		if recv[i] > vectorTime[i] {
			vectorTime[i] = recv[i]
		}
	}

	vectorTime[workerID]++
	return vectorTime
}

func incrementLamportTime() int {
	mu.Lock()
	defer mu.Unlock()

	lamportTime++
	return lamportTime
}

func incrementVectorTime() []int {
	mu.Lock()
	defer mu.Unlock()

	vectorTime[workerID]++
	return vectorTime
}

func lamportHandler(w http.ResponseWriter, r *http.Request) {

	recvClockStr := r.Header.Get("X-Clock")
	recvClock, _ := strconv.Atoi(recvClockStr)

	currentTime := updateLamportTime(recvClock)

	aStr := r.URL.Query().Get("a")
	bStr := r.URL.Query().Get("b")
	op := r.URL.Query().Get("op")

	a, err1 := strconv.Atoi(aStr)
	b, err2 := strconv.Atoi(bStr)

	if err1 != nil || err2 != nil {
		http.Error(w, "invalid numbers", http.StatusBadRequest)
		return
	}

	var result int

	switch op {
	case "add":
		result = a + b
	default:
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}

	fmt.Printf("Worker time: %d | result: %d\n", currentTime, result)

	sendTime := incrementLamportTime()

	w.Header().Set("X-Clock", strconv.Itoa(sendTime))
	fmt.Fprintf(w, "Result: %d", result)
}

func vectorHandler(w http.ResponseWriter, r *http.Request) {
	clockFromHeader := r.Header.Get("X-Clock")

	var recv []int
	json.Unmarshal([]byte(clockFromHeader), &recv)

	currentTime := updateVectorTime(recv)

	aStr := r.URL.Query().Get("a")
	bStr := r.URL.Query().Get("b")
	op := r.URL.Query().Get("op")

	a, err1 := strconv.Atoi(aStr)
	b, err2 := strconv.Atoi(bStr)

	if err1 != nil || err2 != nil {
		http.Error(w, "invalid numbers", http.StatusBadRequest)
		return
	}

	var result int

	switch op {
	case "add":
		result = a + b
	default:
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}

	timeBytes, err := json.Marshal(currentTime)
	if err != nil {
		http.Error(w, "failed to marshal clock", http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-Clock", string(timeBytes))
	fmt.Fprintf(w, "Result: %d", result)
}

func startWorker(port int, clockType string, id int, nodes int) {

	workerID = id
	vectorTime = make([]int, nodes)

	if clockType == "lamport" {
		http.HandleFunc("/lamport", lamportHandler)
	} else {
		http.HandleFunc("/vector", vectorHandler)
	}

	addr := fmt.Sprintf(":%d", port)

	fmt.Printf("Worker running on %s (id=%d, clock=%s)\n", addr, id, clockType)

	http.ListenAndServe(addr, nil)
}
