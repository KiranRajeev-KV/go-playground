package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Println("----- upstream received request -----")
	log.Printf("method: %s", r.Method)
	log.Printf("path: %s", r.URL.Path)
	log.Printf("query: %s", r.URL.RawQuery)
	log.Printf("host: %s", r.Host)
	log.Printf("remote addr: %s", r.RemoteAddr)

	for key, values := range r.Header {
		for _, value := range values {
			log.Printf("header: %s=%s", key, value)
		}
	}

	log.Printf("body: %s", string(body))
	log.Println("-------------------------------------")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Upstream-Test", "hello")
	w.WriteHeader(http.StatusCreated)

	fmt.Fprintf(
		w,
		`{"message":"received","method":%q,"path":%q}`,
		r.Method,
		r.URL.RequestURI(),
	)
}

func main() {
	http.HandleFunc("/", handler)

	log.Println("upstream listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}