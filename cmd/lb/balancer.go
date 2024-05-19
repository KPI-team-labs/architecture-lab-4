package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/KPI-team-labs/architecture-lab-4/httptools"
	"github.com/KPI-team-labs/architecture-lab-4/signal"
)

type Server struct {
	URLPath         string
	ConnectionCount int
	IsHealthy       bool
}

var (
	port       = flag.Int("port", 8090, "load balancer port")
	timeoutSec = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https      = flag.Bool("https", false, "whether backends support HTTPs")

	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

var (
	timeout     = time.Duration(*timeoutSec) * time.Second
	serversPool = []*Server{
		{URLPath: "server1:8080"},
		{URLPath: "server2:8080"},
		{URLPath: "server3:8080"},
	}
	mutex sync.Mutex
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

func health(server *Server) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s://%s/health", scheme(), server.URLPath), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	server.IsHealthy = true
	return true
}

func minConnectionServerIndex(serversPool []*Server) int {
	minIndex := -1
	minConnectionCount := -1

	for index, serverObj := range serversPool {
		if serverObj.IsHealthy {
			if minIndex == -1 || serverObj.ConnectionCount < minConnectionCount {
				minIndex = index
				minConnectionCount = serverObj.ConnectionCount
			}
		}
	}

	return minIndex
}

func forward(rw http.ResponseWriter, r *http.Request) error {
	ctx, _ := context.WithTimeout(r.Context(), timeout)
	fwdRequest := r.Clone(ctx)
	mutex.Lock()
	minIndex := minConnectionServerIndex(serversPool)

	if minIndex == -1 {
		mutex.Unlock()
		rw.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("There are no healthy servers")
	}
	destination := serversPool[minIndex]
	destination.ConnectionCount++
	mutex.Unlock()

	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = destination.URLPath
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = destination.URLPath

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}
		if *traceEnabled {
			rw.Header().Set("lb-from", destination.URLPath)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		_, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
		return nil
	} else {
		log.Printf("Failed to get response from %s: %s", destination.URLPath, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
}

func main() {
	flag.Parse()

	for _, server := range serversPool {
		server.IsHealthy = health(server)
		go func(serverObj *Server) {
			for range time.Tick(10 * time.Second) {
				mutex.Lock()
				serverObj.IsHealthy = health(serverObj)
				log.Printf("%s: health=%t, connCnt=%d", serverObj.URLPath, serverObj.IsHealthy, serverObj.ConnectionCount)
				mutex.Unlock()
			}
		}(server)
	}

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		forward(rw, r)
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}
