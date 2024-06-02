package integration

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

const baseAddress = "http://balancer:8090"
const key = "mcqueen-team"
const numRequests = 10

type RespBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var client = http.Client{
	Timeout: 3 * time.Second,
}

type IntegrationTestSuite struct{}

var _ = Suite(&IntegrationTestSuite{})

func TestBalancer(t *testing.T) {
	TestingT(t)
}

func sendRequest(baseAddress, key string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/some-data?key=%s", baseAddress, key), nil)
	if err != nil {
		log.Printf("error creating request: %s", err)
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error: %s", err)
		return nil, err
	}
	log.Printf("response %d", resp.StatusCode)
	return resp, nil
}

func (s *IntegrationTestSuite) TestGetRequest(c *C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	for i := 0; i < numRequests; i++ {
		resp, err := sendRequest(baseAddress, key, &client)
		if err != nil {
			c.Error(err)
		}
		defer resp.Body.Close()

		switch i % 3 {
		case 0:
			c.Check(resp.Header.Get("lb-from"), Equals, "server1:8080")
		case 1:
			c.Check(resp.Header.Get("lb-from"), Equals, "server2:8080")
		case 2:
			c.Check(resp.Header.Get("lb-from"), Equals, "server3:8080")
		}

		// Validate the response body
		var body RespBody
		err = json.NewDecoder(resp.Body).Decode(&body)
		if err != nil {
			c.Error(err)
		}
		c.Check(body.Key, Equals, key)
		c.Assert(body.Value, Not(Equals), "")
	}

	db, err := client.Get(fmt.Sprintf("%s/api/v1/some-data?key=%s", baseAddress, key))
	if err != nil {
		c.Error(err)
	}
	defer db.Body.Close()

	var body RespBody
	err = json.NewDecoder(db.Body).Decode(&body)
	if err != nil {
		c.Error(err)
	}
	c.Check(body.Key, Equals, key)
	c.Assert(body.Value, Not(Equals), "")
}

func (s *IntegrationTestSuite) BenchmarkBalancer(c *C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	for i := 0; i < c.N; i++ {
		resp, err := sendRequest(baseAddress, key, &client)
		if err != nil {
			c.Error(err)
		}
		defer resp.Body.Close()
	}
}
