package integration

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

type IntegrationTestSuite struct{}

var _ = Suite(&IntegrationTestSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *IntegrationTestSuite) TestGetRequest(c *C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	serverNum := 0
	for i := 0; i < 10; i++ {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data", baseAddress))
		c.Assert(err, IsNil)

		if i%3 == 0 {
			serverNum = 1
		} else if i%3 == 1 {
			serverNum = 2
		} else {
			serverNum = 3
		}
		c.Assert(resp.Header.Get("lb-from"), Equals, fmt.Sprintf("server%d:8080", serverNum))
	}
}

func (s *IntegrationTestSuite) BenchmarkBalancer(c *C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	for i := 0; i < c.N; i++ {
		_, err := client.Get(fmt.Sprintf("%s/api/v1/some-data", baseAddress))
		c.Assert(err, IsNil)
	}
}
