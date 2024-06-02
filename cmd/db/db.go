package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/KPI-team-labs/architecture-lab-4/datastore"
	"github.com/KPI-team-labs/architecture-lab-4/httptools"
	"github.com/KPI-team-labs/architecture-lab-4/signal"
)

var port = flag.Int("port", 8083, "server port")

type RespBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ReqBody struct {
	Value string `json:"value"`
}

type server struct {
	*http.ServeMux
}

func main() {
	flag.Parse()

	s := &server{ServeMux: http.NewServeMux()}
	dir, err := ioutil.TempDir("", "temp-dir")
	if err != nil {
		log.Fatal(err)
	}
	db, err := datastore.NewDb(dir, 250)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	s.HandleFunc("/db/", func(rw http.ResponseWriter, req *http.Request) {
		handleDBRequest(rw, req, db)
	})

	httpServer := httptools.CreateServer(*port, s)
	httpServer.Start()

	signal.WaitForTerminationSignal()
}

func (s *server) Start() {
	log.Printf("Server listening on port %d", *port)
	err := http.ListenAndServe(":"+strconv.Itoa(*port), s)
	if err != nil {
		log.Fatal(err)
	}
}

func handleDBRequest(rw http.ResponseWriter, req *http.Request, Db *datastore.Db) {
	log.Println("Caught request")
	key := req.URL.Path[len("/db/"):]
	log.Printf("Key: %s", key)

	switch req.Method {
	case http.MethodGet:
		value, err := Db.Get(key)
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.WriteHeader(http.StatusOK)
		rw.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(rw).Encode(RespBody{
			Key:   key,
			Value: value,
		})
	case http.MethodPost:
		var body ReqBody

		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		err = Db.Put(key, body.Value)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusCreated)
	default:
		rw.WriteHeader(http.StatusBadRequest)
	}
}
