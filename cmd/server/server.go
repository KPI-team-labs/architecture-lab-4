package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/KPI-team-labs/architecture-lab-4/httptools"
	"github.com/KPI-team-labs/architecture-lab-4/signal"
)

var port = flag.Int("port", 8080, "server port")

const (
	confResponseDelaySec = "CONF_RESPONSE_DELAY_SEC"
	confHealthFailure    = "CONF_HEALTH_FAILURE"
	dbUrl                = "http://db:8083/db"
)

type ReqBody struct {
	Value string `json:"value"`
}

type RespBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	h := new(http.ServeMux)
	client := http.DefaultClient

	h.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "text/plain")
		if failConfig := os.Getenv(confHealthFailure); failConfig == "true" {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("FAILURE"))
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("OK"))
		}
	})

	report := make(Report)

	h.HandleFunc("/api/v1/some-data", func(rw http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key != "" {
			resp, err := client.Get(fmt.Sprintf("%s/%s", dbUrl, key))
			statusOk := resp.StatusCode >= 200 && resp.StatusCode < 300
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !statusOk {
				rw.WriteHeader(resp.StatusCode)
				return
			}

			var body RespBody
			json.NewDecoder(resp.Body).Decode(&body)

			rw.Header().Set("content-type", "application/json")
			rw.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(rw).Encode(body)

			defer resp.Body.Close()
		}
		respDelayString := os.Getenv(confResponseDelaySec)
		if delaySec, parseErr := strconv.Atoi(respDelayString); parseErr == nil && delaySec > 0 && delaySec < 300 {
			time.Sleep(time.Duration(delaySec) * time.Second)
		}

		report.Process(r)

		rw.Header().Set("content-type", "application/json")
		rw.WriteHeader(http.StatusOK)

		responseSize := 1024
		if key == "" {
			if sizeHeader := r.Header.Get("Response-Size"); sizeHeader != "" {
				if size, err := strconv.Atoi(sizeHeader); err == nil && size > 0 {
					responseSize = size
				}
			}

			responseData := make([]string, responseSize)
			for i := 0; i < responseSize; i++ {
				responseData[i] = strconv.Itoa(responseSize)
			}

			_ = json.NewEncoder(rw).Encode(responseData)
		}

	})

	h.Handle("/report", report)

	server := httptools.CreateServer(*port, h)
	server.Start()

	buff := new(bytes.Buffer)
	body := ReqBody{Value: time.Now().Format(time.RFC3339)}
	json.NewEncoder(buff).Encode(body)

	res, err := client.Post(fmt.Sprintf("%s/mcqueen-team", dbUrl), "application/json", buff)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	signal.WaitForTerminationSignal()
}
