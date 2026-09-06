package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	listenAddr  = ":8080"
	upstreamURL = "http://localhost:9000"
	proxyName   = "reverse-proxy"
)

var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

type Proxy struct {
	target    *url.URL
	transport http.RoundTripper
	name      string
}

type noopCloseReadCloser struct {
	io.ReadCloser
}

func (noopCloseReadCloser) Close() error {
	return nil
}

func NewProxy(
	target *url.URL,
	transport http.RoundTripper,
	name string,
) (*Proxy, error) {
	switch {
	case target == nil:
		return nil, errors.New("proxy target is required")
	case target.Scheme != "http" && target.Scheme != "https":
		return nil, fmt.Errorf("unsupported proxy target scheme %q", target.Scheme)
	case target.Host == "":
		return nil, errors.New("proxy target host is required")
	case name == "":
		return nil, errors.New("proxy name is required")
	}

	if transport == nil {
		transport = newProxyTransport()
	}

	return &Proxy{
		target:    target.Clone(),
		transport: transport,
		name:      name,
	}, nil
}

func newProxyTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// A proxy should not have Go silently inject Accept-Encoding: gzip
	// and transparently decompress the upstream response.
	transport.DisableCompression = true

	return transport
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf(
		"method=%s path=%s host=%s proto=%s",
		r.Method,
		r.URL.Path,
		r.Host,
		r.Proto,
	)

	outReq := r.Clone(r.Context())

	rewriteRequestURL(outReq.URL, p.target)

	// RequestURI is only valid on an incoming server request.
	// For an outgoing client request it must be empty.
	outReq.RequestURI = ""

	// Let the outbound URL determine the Host header.
	// The original host is preserved separately in X-Forwarded-Host.
	outReq.Host = ""

	// The client->proxy connection lifecycle must not force the
	// proxy->upstream connection closed.
	outReq.Close = false

	if r.ContentLength == 0 {
		outReq.Body = nil
	} else if outReq.Body != nil {
		// Request.Clone shallow-copies Body. RoundTrip owns and closes the
		// outbound body, so prevent it from closing the server-owned inbound
		// request body.
		outReq.Body = noopCloseReadCloser{ReadCloser: outReq.Body}
		defer outReq.Body.Close()
	}

	// Request trailers are intentionally not supported yet.
	outReq.Trailer = nil

	if outReq.Header == nil {
		outReq.Header = make(http.Header)
	}

	removeHopByHopHeaders(outReq.Header)
	setForwardedHeaders(outReq, r)
	appendVia(outReq.Header, r.ProtoMajor, r.ProtoMinor, p.name)

	// Prevent net/http from synthesizing Go-http-client/... when the
	// original client sent no User-Agent.
	if _, ok := outReq.Header["User-Agent"]; !ok {
		outReq.Header.Set("User-Agent", "")
	}

	resp, err := p.transport.RoundTrip(outReq)
	if err != nil {
		log.Printf("upstream request failed: %v", err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	responseHeaders := resp.Header.Clone()

	removeHopByHopHeaders(responseHeaders)
	appendVia(
		responseHeaders,
		resp.ProtoMajor,
		resp.ProtoMinor,
		p.name,
	)

	copyHeaders(w.Header(), responseHeaders)

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("failed to copy upstream response: %v", err)

		// The response may already be partially written, so sending another
		// HTTP status is impossible. Tell net/http to abort the response.
		panic(http.ErrAbortHandler)
	}
}

func rewriteRequestURL(reqURL, target *url.URL) {
	targetQuery := target.RawQuery

	reqURL.Scheme = target.Scheme
	reqURL.Host = target.Host
	reqURL.Path, reqURL.RawPath = joinURLPath(target, reqURL)

	switch {
	case targetQuery == "":
	case reqURL.RawQuery == "":
		reqURL.RawQuery = targetQuery
	default:
		reqURL.RawQuery = targetQuery + "&" + reqURL.RawQuery
	}

	reqURL.ForceQuery = target.ForceQuery || reqURL.ForceQuery
}

func joinURLPath(a, b *url.URL) (path, rawPath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}

	aPath := a.EscapedPath()
	bPath := b.EscapedPath()

	aSlash := strings.HasSuffix(aPath, "/")
	bSlash := strings.HasPrefix(bPath, "/")

	switch {
	case aSlash && bSlash:
		return a.Path + b.Path[1:], aPath + bPath[1:]

	case !aSlash && !bSlash:
		return a.Path + "/" + b.Path, aPath + "/" + bPath

	default:
		return a.Path + b.Path, aPath + bPath
	}
}

func singleJoiningSlash(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")

	switch {
	case aSlash && bSlash:
		return a + b[1:]

	case !aSlash && !bSlash:
		return a + "/" + b

	default:
		return a + b
	}
}

func removeHopByHopHeaders(h http.Header) {
	for _, connectionValue := range h.Values("Connection") {
		for headerName := range strings.SplitSeq(connectionValue, ",") {
			headerName = textproto.TrimString(headerName)

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
	// These values came from an untrusted client, so do not preserve them.
	outReq.Header.Del("Forwarded")
	outReq.Header.Del("X-Forwarded-For")
	outReq.Header.Del("X-Forwarded-Host")
	outReq.Header.Del("X-Forwarded-Proto")

	if clientIP, _, err := net.SplitHostPort(inReq.RemoteAddr); err == nil {
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	outReq.Header.Set("X-Forwarded-Host", inReq.Host)

	if inReq.TLS != nil {
		outReq.Header.Set("X-Forwarded-Proto", "https")
	} else {
		outReq.Header.Set("X-Forwarded-Proto", "http")
	}
}

func appendVia(
	h http.Header,
	protoMajor int,
	protoMinor int,
	proxyName string,
) {
	via := fmt.Sprintf(
		"%d.%d %s",
		protoMajor,
		protoMinor,
		proxyName,
	)

	h.Add("Via", via)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func main() {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		log.Fatalf("parse upstream URL: %v", err)
	}

	proxy, err := NewProxy(target, nil, proxyName)
	if err != nil {
		log.Fatalf("create proxy: %v", err)
	}

	mux := http.NewServeMux()

	// Deliberately matches every method and every path.
	mux.Handle("/", proxy)

	server := &http.Server{
		Addr:                listenAddr,
		Handler:             mux,
		ReadHeaderTimeout:   5 * time.Second,
		IdleTimeout:         2 * time.Minute,
		MaxHeaderValueCount: http.DefaultMaxHeaderValueCount,
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		log.Printf(
			"starting reverse proxy on %s -> %s",
			listenAddr,
			target,
		)

		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
		return

	case <-signalCtx.Done():
		log.Println("shutting down reverse proxy")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)

		if closeErr := server.Close(); closeErr != nil {
			log.Printf("forced server close failed: %v", closeErr)
		}
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP server stopped with error: %v", err)
	}
}
