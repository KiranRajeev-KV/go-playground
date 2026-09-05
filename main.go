package main

import (
	"io"
	"log"
	"net/http"
)

var client = &http.Client{}

func handleReq(w http.ResponseWriter, r *http.Request) {
	log.Printf(
		"method=%s path=%s host=%s",
		r.Method,
		r.URL.Path,
		r.Host,
	)

	upstreamURL := "http://localhost:9000" + r.URL.RequestURI()

	upstreamReq, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		upstreamURL,
		r.Body,
	)
	if err != nil {
		log.Printf("failed to create upstream request: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(upstreamReq)
	if err != nil {
		log.Printf("upstream request failed: %v", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("failed to copy upstream response: %v", err)
	}
}

func main() {
	http.HandleFunc("/", handleReq)

	log.Println("starting server on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
