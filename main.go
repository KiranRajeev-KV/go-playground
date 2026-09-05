package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

var client = &http.Client{
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"HTTP2-Settings",
}

const proxyName = "reverse-proxy"

func removeHopByHopHeaders(h http.Header) {
	// RFC 9110: fields named by Connection are hop-by-hop as well.
	for _, connectionValue := range h.Values("Connection") {
		for _, headerName := range strings.Split(connectionValue, ",") {
			headerName = strings.TrimSpace(headerName)
			if headerName != "" {
				h.Del(headerName)
			}
		}
	}

	for _, headerName := range hopByHopHeaders {
		h.Del(headerName)
	}
}

func setForwardedHeaders(outReq, inReq *http.Request) {
	// Do not trust forwarding metadata supplied directly by the client.
	outReq.Header.Del("Forwarded")
	outReq.Header.Del("X-Forwarded-For")
	outReq.Header.Del("X-Forwarded-Host")
	outReq.Header.Del("X-Forwarded-Proto")

	clientIP, _, err := net.SplitHostPort(inReq.RemoteAddr)
	if err == nil {
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	outReq.Header.Set("X-Forwarded-Host", inReq.Host)

	if inReq.TLS != nil {
		outReq.Header.Set("X-Forwarded-Proto", "https")
	} else {
		outReq.Header.Set("X-Forwarded-Proto", "http")
	}
}

func appendVia(h http.Header, protoMajor, protoMinor int) {
	receivedProtocol := fmt.Sprintf("%d.%d", protoMajor, protoMinor)
	entry := receivedProtocol + " " + proxyName

	h.Add("Via", entry)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func handleReq(w http.ResponseWriter, r *http.Request) {
	log.Printf(
		"method=%s path=%s host=%s proto=%s",
		r.Method,
		r.URL.Path,
		r.Host,
		r.Proto,
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

	upstreamReq.Header = r.Header.Clone()

	// Preserve known inbound body length. Content-Length is represented
	// structurally by Request.ContentLength, not by the generic Header map.
	upstreamReq.ContentLength = r.ContentLength

	removeHopByHopHeaders(upstreamReq.Header)
	setForwardedHeaders(upstreamReq, r)
	appendVia(
		upstreamReq.Header,
		r.ProtoMajor,
		r.ProtoMinor,
	)

	resp, err := client.Do(upstreamReq)
	if err != nil {
		log.Printf("upstream request failed: %v", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	responseHeaders := resp.Header.Clone()

	removeHopByHopHeaders(responseHeaders)

	// This proxy is forwarding the upstream response, so append its
	// intermediary entry to the response Via chain as well.
	appendVia(
		responseHeaders,
		resp.ProtoMajor,
		resp.ProtoMinor,
	)

	copyHeaders(w.Header(), responseHeaders)

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("failed to copy upstream response: %v", err)
	}
}

func main() {
	http.HandleFunc("/", handleReq)

	log.Println("starting reverse proxy on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
