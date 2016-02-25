package dnsdisco_test

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/rafaeljusto/dnsdisco"
)

func TestDiscover(t *testing.T) {
	scenarios := []struct {
		description    string
		service        string
		proto          string
		name           string
		expectedTarget string
		expectedPort   uint16
		expectedError  error
	}{
		{
			description:    "it should retrieve the target correctly",
			service:        "jabber",
			proto:          "tcp",
			name:           "registro.br",
			expectedTarget: "jabber.registro.br.",
			expectedPort:   5269,
		},
		{
			description: "it should fail when the protocol is invalid",
			service:     "jabber",
			proto:       "xxx",
			name:        "registro.br",
			expectedError: &net.DNSError{
				Err:  "no such host",
				Name: "_jabber._xxx.registro.br",
			},
		},
	}

	for i, item := range scenarios {
		target, port, err := dnsdisco.Discover(item.service, item.proto, item.name)

		if target != item.expectedTarget {
			t.Errorf("scenario %d, “%s”: mismatch targets. Expecting: “%s”; found “%s”",
				i, item.description, item.expectedTarget, target)
		}

		if port != item.expectedPort {
			t.Errorf("scenario %d, “%s”: mismatch ports. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedPort, port)
		}

		// As the resolver change between machines, we can't guess the DNSError name's attribute. So we
		// need to inject the value on the expected error
		dnsError, ok1 := err.(*net.DNSError)
		expectedDNSError, ok2 := item.expectedError.(*net.DNSError)

		if ok1 && ok2 {
			expectedDNSError.Server = dnsError.Server
		}

		if !reflect.DeepEqual(err, item.expectedError) {
			t.Errorf("scenario %d, “%s”: mismatch errors. Expecting: “%v”; found “%v”",
				i, item.description, item.expectedError, err)
		}
	}
}

func TestDiscoverDefaultBalancer(t *testing.T) {
	scenarios := []struct {
		description    string
		service        string
		proto          string
		name           string
		retriever      dnsdisco.RetrieverFunc
		healthChecker  dnsdisco.HealthCheckerFunc
		rerun          int
		expectedTarget string
		expectedPort   uint16
	}{
		{
			description: "it should retrieve the target correctly (fallback inside priority group)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server2.example.com.", nil
			}),
			expectedTarget: "server2.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should retrieve the target correctly (fallback to other priority group by health check)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
					{
						Target:   "server3.example.com.",
						Port:     3333,
						Priority: 20,
						Weight:   20,
					},
					{
						Target:   "server4.example.com.",
						Port:     4444,
						Priority: 20,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server4.example.com.", nil
			}),
			expectedTarget: "server4.example.com.",
			expectedPort:   4444,
		},
		{
			description: "it should retrieve the target correctly (fallback to other priority group by used counter)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
					{
						Target:   "server3.example.com.",
						Port:     3333,
						Priority: 20,
						Weight:   20,
					},
					{
						Target:   "server4.example.com.",
						Port:     4444,
						Priority: 20,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target != "server3.example.com.", nil
			}),
			rerun:          2,
			expectedTarget: "server4.example.com.",
			expectedPort:   4444,
		},
		{
			description: "it should retrieve the target correctly (use the less used server)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   0,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				switch target {
				case "server1.example.com.":
					return true, nil
				case "server2.example.com.":
					return true, nil
				}

				return false, nil
			}),
			rerun:          1,
			expectedTarget: "server2.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should retrieve the target correctly (same target different port)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   0,
					},
					{
						Target:   "server1.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   200,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server1.example.com.", nil
			}),
			expectedTarget: "server1.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should not select any target",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   0,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return false, nil
			}),
			expectedTarget: "",
			expectedPort:   0,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetHealthChecker(item.healthChecker)

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		var target string
		var port uint16

		for j := 0; j <= item.rerun; j++ {
			target, port = discovery.Choose()
		}

		if target != item.expectedTarget {
			t.Errorf("scenario %d, “%s”: mismatch targets. Expecting: “%s”; found “%s”",
				i, item.description, item.expectedTarget, target)
		}

		if port != item.expectedPort {
			t.Errorf("scenario %d, “%s”: mismatch ports. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedPort, port)
		}
	}
}

func TestDiscoverDefaultHealthChecker(t *testing.T) {
	ln, err := startTCPTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	testServerHost, p, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	testServerPort, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		description    string
		service        string
		proto          string
		name           string
		retriever      dnsdisco.RetrieverFunc
		balancer       dnsdisco.BalancerFunc
		expectedTarget string
		expectedPort   uint16
		expectedError  error
	}{
		{
			description: "it should identify a healthy server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   testServerHost,
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			balancer: dnsdisco.BalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			expectedTarget: testServerHost,
			expectedPort:   uint16(testServerPort),
		},
		{
			description: "it should fail when it's not a valid proto",
			service:     "jabber",
			proto:       "xxx",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   testServerHost,
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			balancer: dnsdisco.BalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			expectedTarget: "",
			expectedPort:   0,
		},
		{
			description: "it should fail to connect to an unknown server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "idontexist.example.com.",
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			balancer: dnsdisco.BalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			expectedTarget: "",
			expectedPort:   0,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetBalancer(item.balancer)

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		target, port := discovery.Choose()

		if target != item.expectedTarget {
			t.Errorf("scenario %d, “%s”: mismatch targets. Expecting: “%s”; found “%s”",
				i, item.description, item.expectedTarget, target)
		}

		if port != item.expectedPort {
			t.Errorf("scenario %d, “%s”: mismatch ports. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedPort, port)
		}
	}
}

func TestDiscoverHealthCheckerTTL(t *testing.T) {
	ln, err := startTCPTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	testServerHost, p, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	testServerPort, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		description      string
		service          string
		proto            string
		name             string
		retriever        dnsdisco.RetrieverFunc
		healthCheckerTTL time.Duration
		balancer         dnsdisco.BalancerFunc
		rerun            int
		expectedCalls    int
	}{
		{
			description: "it should avoid calling the health check more than once in the TTL period",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   testServerHost,
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			healthCheckerTTL: 1 * time.Second,
			balancer: dnsdisco.BalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			rerun:         1,
			expectedCalls: 1,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetHealthCheckerTTL(item.healthCheckerTTL)
		discovery.SetBalancer(item.balancer)

		calls := 0
		discovery.SetHealthChecker(dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
			calls++
			return true, nil
		}))

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		for j := 0; j <= item.rerun; j++ {
			discovery.Choose()
		}

		if calls != item.expectedCalls {
			t.Errorf("scenario %d, “%s”: mismatch health check calls. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedCalls, calls)
		}
	}
}

func TestRefreshAsync(t *testing.T) {
	scenarios := []struct {
		description     string
		service         string
		proto           string
		name            string
		refreshInterval time.Duration
		retriever       dnsdisco.RetrieverFunc
		healthChecker   dnsdisco.HealthCheckerFunc
		expectedTarget  string
		expectedPort    uint16
		expectedErrors  []error
	}{
		{
			description:     "it should update the servers asynchronously",
			service:         "jabber",
			proto:           "tcp",
			name:            "registro.br",
			refreshInterval: 100 * time.Millisecond,
			retriever: func() dnsdisco.RetrieverFunc {
				calls := 0

				return dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
					calls++
					if calls == 1 {
						return []*net.SRV{
							{
								Target:   "server1.example.com.",
								Port:     1111,
								Priority: 10,
								Weight:   20,
							},
							{
								Target:   "server2.example.com.",
								Port:     2222,
								Priority: 10,
								Weight:   10,
							},
						}, nil
					}

					return []*net.SRV{
						{
							Target:   "server3.example.com.",
							Port:     3333,
							Priority: 15,
							Weight:   20,
						},
						{
							Target:   "server4.example.com.",
							Port:     4444,
							Priority: 10,
							Weight:   10,
						},
					}, nil
				})
			}(),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return true, nil
			}),
			expectedTarget: "server4.example.com.",
			expectedPort:   4444,
		},
		{
			description:     "it should fail to retrieve the SRV records",
			service:         "jabber",
			proto:           "tcp",
			name:            "registro.br",
			refreshInterval: 100 * time.Millisecond,
			retriever: func() dnsdisco.RetrieverFunc {
				calls := 0

				return dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
					calls++
					if calls == 1 {
						return []*net.SRV{
							{
								Target:   "server1.example.com.",
								Port:     1111,
								Priority: 10,
								Weight:   100,
							},
							{
								Target:   "server2.example.com.",
								Port:     2222,
								Priority: 10,
								Weight:   0,
							},
						}, nil
					}

					return nil, net.UnknownNetworkError("test")
				})
			}(),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return true, nil
			}),
			expectedTarget: "server1.example.com.",
			expectedPort:   1111,
			expectedErrors: []error{
				net.UnknownNetworkError("test"),
			},
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetHealthChecker(item.healthChecker)

		finish := discovery.RefreshAsync(item.refreshInterval)
		time.Sleep(item.refreshInterval + (50 * time.Millisecond))

		target, port := discovery.Choose()

		if target != item.expectedTarget {
			t.Errorf("scenario %d, “%s”: mismatch targets. Expecting: “%s”; found “%s”",
				i, item.description, item.expectedTarget, target)
		}

		if port != item.expectedPort {
			t.Errorf("scenario %d, “%s”: mismatch ports. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedPort, port)
		}

		if errs := discovery.Errors(); !reflect.DeepEqual(errs, item.expectedErrors) {
			t.Errorf("scenario %d, “%s”: mismatch errors. Expecting: “%#v”; found “%#v”",
				i, item.description, item.expectedErrors, errs)
		}

		close(finish)
	}
}

func ExampleDiscover() {
	target, port, err := dnsdisco.Discover("jabber", "tcp", "registro.br")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Target: %s\nPort: %d\n", target, port)

	// Output:
	// Target: jabber.registro.br.
	// Port: 5269
}

func ExampleDiscovery_RefreshAsync() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")

	// depending on where this examples run the retrieving time differs (DNS RTT),
	// so as we cannot sleep a deterministic period, to make this test more useful
	// we are creating a channel to alert the main go routine that we got an
	// answer from the network
	retrieved := make(chan bool)

	discovery.SetRetriever(dnsdisco.RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
		_, servers, err = net.LookupSRV(service, proto, name)
		retrieved <- true
		return
	}))

	// refresh the SRV records every 100 milliseconds
	stopRefresh := discovery.RefreshAsync(100 * time.Millisecond)
	<-retrieved

	// sleep for a short period only to allow the library to process the SRV
	// records retrieved from the network
	time.Sleep(100 * time.Millisecond)

	target, port := discovery.Choose()
	fmt.Printf("Target: %s\nPort: %d\n", target, port)
	close(stopRefresh)

	// Output:
	// Target: jabber.registro.br.
	// Port: 5269
}

// ExampleRetrieverFunc uses a specific resolver with custom timeouts
func ExampleRetrieverFunc() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetRetriever(dnsdisco.RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
		client := dns.Client{
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		}

		name = strings.TrimRight(name, ".")
		z := fmt.Sprintf("_%s._%s.%s.", service, proto, name)

		var request dns.Msg
		request.SetQuestion(z, dns.TypeSRV)
		request.RecursionDesired = true

		response, _, err := client.Exchange(&request, "8.8.8.8:53")
		if err != nil {
			return nil, err
		}

		for _, rr := range response.Answer {
			if srv, ok := rr.(*dns.SRV); ok {
				servers = append(servers, &net.SRV{
					Target:   srv.Target,
					Port:     srv.Port,
					Priority: srv.Priority,
					Weight:   srv.Weight,
				})
			}
		}

		return
	}))

	// Retrieve the servers
	if err := discovery.Refresh(); err != nil {
		fmt.Println(err)
		return
	}

	target, port := discovery.Choose()
	fmt.Printf("Target: %s\nPort: %d\n", target, port)

	// Output:
	// Target: jabber.registro.br.
	// Port: 5269
}

// ExampleBalancerFunc uses a round-robin algorithm. As we don't known which
// server position was used in the last time, we try to select using the Used
// attribute.
func ExampleBalancerFunc() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetBalancer(dnsdisco.BalancerFunc(func(servers []dnsdisco.Server) (index int) {
		minimum := -1
		for _, server := range servers {
			if server.Used < minimum || minimum == -1 {
				minimum = server.Used
			}
		}

		for i, server := range servers {
			if server.Used == minimum {
				return i
			}
		}

		return -1
	}))

	// Retrieve the servers
	if err := discovery.Refresh(); err != nil {
		fmt.Println(err)
		return
	}

	target, port := discovery.Choose()
	fmt.Printf("Target: %s\nPort: %d\n", target, port)

	// Output:
	// Target: jabber.registro.br.
	// Port: 5269
}

// ExampleHealthCheckerFunc tests HTTP fetching the homepage and checking the
// HTTP status code.
func ExampleHealthCheckerFunc() {
	discovery := dnsdisco.NewDiscovery("http", "tcp", "pantz.org")
	discovery.SetHealthChecker(dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
		response, err := http.Get("http://www.pantz.org")
		if err != nil {
			return false, err
		}

		return response.StatusCode == http.StatusOK, nil
	}))

	// Retrieve the servers
	if err := discovery.Refresh(); err != nil {
		fmt.Println(err)
		return
	}

	target, port := discovery.Choose()
	fmt.Printf("Target: %s\nPort: %d\n", target, port)

	// Output:
	// Target: www.pantz.org.
	// Port: 80
}

func BenchmarkBalancer(b *testing.B) {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetHealthChecker(dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
		return true, nil
	}))

	discovery.SetRetriever(dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
		return []*net.SRV{
			{
				Target:   "server1.example.com.",
				Port:     1111,
				Weight:   10,
				Priority: 20,
			},
			{
				Target:   "server2.example.com.",
				Port:     2222,
				Weight:   70,
				Priority: 10,
			},
			{
				Target:   "server3.example.com.",
				Port:     3333,
				Weight:   100,
				Priority: 20,
			},
			{
				Target:   "server4.example.com.",
				Port:     4444,
				Weight:   1,
				Priority: 15,
			},
			{
				Target:   "server5.example.com.",
				Port:     5555,
				Weight:   40,
				Priority: 60,
			},
		}, nil
	}))

	// Retrieve the servers
	if err := discovery.Refresh(); err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < b.N; i++ {
		discovery.Choose()
	}
}

// startTCPTestServer initialize a TCP echo server running on any available port
// of the localhost. The returning listener must be closed to terminate the
// server.
func startTCPTestServer() (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				break
			}
			// Server connection.
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)

				n, err := c.Read(buf)
				if err != nil {
					return
				}
				c.Write(buf[:n])
			}(c)
		}
	}()

	return ln, nil
}
