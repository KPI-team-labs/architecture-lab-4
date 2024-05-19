package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jarcoal/httpmock"
	. "gopkg.in/check.v1"
)

func TestBalancer(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestScheme(c *C) {
	*https = false
	c.Assert(scheme(), Equals, "http")

	*https = true
	c.Assert(scheme(), Equals, "https")

	*https = false
}

func (s *MySuite) TestFindBestServer(c *C) {
	// No healthy servers
	serversPool = []*Server{
		{URLPath: "Server1", ConnectionCount: 10, IsHealthy: false},
		{URLPath: "Server2", ConnectionCount: 20, IsHealthy: false},
		{URLPath: "Server3", ConnectionCount: 30, IsHealthy: false},
	}
	c.Assert(minConnectionServerIndex(serversPool), Equals, -1)

	// All healthy servers
	serversPool = []*Server{
		{URLPath: "Server1", ConnectionCount: 10, IsHealthy: true},
		{URLPath: "Server2", ConnectionCount: 20, IsHealthy: true},
		{URLPath: "Server3", ConnectionCount: 30, IsHealthy: true},
	}
	c.Assert(minConnectionServerIndex(serversPool), Equals, 0)

	// Mixed healthy and unhealthy servers
	serversPool = []*Server{
		{URLPath: "Server1", ConnectionCount: 10, IsHealthy: false},
		{URLPath: "Server2", ConnectionCount: 20, IsHealthy: true},
		{URLPath: "Server3", ConnectionCount: 30, IsHealthy: true},
	}
	c.Assert(minConnectionServerIndex(serversPool), Equals, 1)

	// Minimum connection count
	serversPool = []*Server{
		{URLPath: "Server1", ConnectionCount: 10, IsHealthy: true},
		{URLPath: "Server2", ConnectionCount: 5, IsHealthy: true},
		{URLPath: "Server3", ConnectionCount: 30, IsHealthy: true},
	}
	c.Assert(minConnectionServerIndex(serversPool), Equals, 1)
}

func (s *MySuite) TestHealth(c *C) {
	mockURL := "http://example.com/health"
	httpmock.RegisterResponder(http.MethodGet, mockURL, httpmock.NewStringResponder(http.StatusOK, ""))

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	server := &Server{
		URLPath: "example.com",
	}

	result := health(server)

	c.Assert(result, Equals, true)
	c.Assert(server.IsHealthy, Equals, true)

	server.IsHealthy = false

	httpmock.RegisterResponder(http.MethodGet, mockURL, httpmock.NewStringResponder(http.StatusInternalServerError, ""))
	result2 := health(server)

	c.Assert(result2, Equals, false)
	c.Assert(server.IsHealthy, Equals, false)
}

func (s *MySuite) TestForward(c *C) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://server1:8080/",
		httpmock.NewStringResponder(200, "OK"))

	serversPool = []*Server{
		{URLPath: "server1:8080", IsHealthy: true},
	}

	req, err := http.NewRequest("GET", "/", nil)
	c.Assert(err, IsNil)
	rr := httptest.NewRecorder()
	err = forward(rr, req)
	c.Assert(err, IsNil)
}

func (s *MySuite) TestForwardWithUnhealthyServer(c *C) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://server1:8080/",
		httpmock.NewStringResponder(500, "Error"))

	serversPool = []*Server{
		{URLPath: "server1:8080", IsHealthy: false},
	}

	req, err := http.NewRequest("GET", "/", nil)
	c.Assert(err, IsNil)
	rr := httptest.NewRecorder()
	err = forward(rr, req)
	c.Assert(err, NotNil)
}
