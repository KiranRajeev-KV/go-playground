package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

var (
	masterLamportTime int
	masterVectorTime  []int
	masterNodeID      int
	numNodes          int
)

type task struct {
	a  int
	b  int
	op string
}

func updateMasterLamportTime(recv int) {
	if recv > masterLamportTime {
		masterLamportTime = recv
	}
	masterLamportTime++
}

func incrementMasterLamportTime() int {
	masterLamportTime++
	return masterLamportTime
}

func updateMasterVectorTime(recv []int) {
	for i := range recv {
		if recv[i] > masterVectorTime[i] {
			masterVectorTime[i] = recv[i]
		}
	}
	masterVectorTime[masterNodeID]++
}

func incrementMasterVectorTime() []int {
	masterVectorTime[masterNodeID]++
	return masterVectorTime
}

func initClock(clockType string, nodes int) {
	numNodes = nodes
	masterLamportTime = 0
	masterVectorTime = make([]int, nodes)
	masterNodeID = 0
}

func startMaster(clockType string, workers []string) {

	tasks := []task{
		{a: 1, b: 2, op: "add"},
		{a: 10, b: 20, op: "add"},
	}

	endpoint := "/lamport"
	if clockType == "vector" {
		endpoint = "/vector"
	}

	for _, w := range workers {
		for _, t := range tasks {

			var sendHeader string

			if clockType == "lamport" {
				incrementMasterLamportTime()
				sendHeader = strconv.Itoa(masterLamportTime)
			} else {
				incTime := incrementMasterVectorTime()
				bytes, _ := json.Marshal(incTime)
				sendHeader = string(bytes)
			}

			url := fmt.Sprintf("%s%s?a=%d&b=%d&op=%s",
				w, endpoint, t.a, t.b, t.op)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				fmt.Println("request error:", err)
				continue
			}

			req.Header.Set("X-Clock", sendHeader)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println("worker error:", err)
				continue
			}

			body, _ := io.ReadAll(res.Body)
			res.Body.Close()

			workerClockStr := res.Header.Get("X-Clock")

			if clockType == "lamport" {
				workerClock, _ := strconv.Atoi(workerClockStr)
				updateMasterLamportTime(workerClock)
				fmt.Printf("Master time: %d | Response: %s\n",
					masterLamportTime, string(body))
			} else {
				var workerClock []int
				json.Unmarshal([]byte(workerClockStr), &workerClock)
				updateMasterVectorTime(workerClock)
				fmt.Printf("Master time: %v | Response: %s\n",
					masterVectorTime, string(body))
			}
		}
	}
}
