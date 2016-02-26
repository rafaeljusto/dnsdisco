package dnsdisco_test

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
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

func TestHealthCheckerTTL(t *testing.T) {
	scenarios := []struct {
		description        string
		service            string
		proto              string
		name               string
		retriever          dnsdisco.RetrieverFunc
		healthCheckerTTL   time.Duration
		healthCheckerError error
		loadBalancer       dnsdisco.LoadBalancerFunc
		rerun              int
		expectedCalls      int
		expectedErrors     []error
	}{
		{
			description: "it should avoid calling the health check more than once in the TTL period",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "example.com",
						Port:     uint16(1111),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			healthCheckerTTL: 100 * time.Millisecond,
			loadBalancer: dnsdisco.LoadBalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			rerun:         1,
			expectedCalls: 2,
		},
		{
			description: "it should detect a health check error",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "example.com",
						Port:     uint16(1111),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			healthCheckerTTL:   100 * time.Millisecond,
			healthCheckerError: fmt.Errorf("error example"),
			loadBalancer: dnsdisco.LoadBalancerFunc(func(servers []dnsdisco.Server) (index int) {
				for i, server := range servers {
					if server.LastHealthCheck {
						return i
					}
				}

				return -1
			}),
			expectedCalls:  2,
			expectedErrors: []error{fmt.Errorf("error example"), fmt.Errorf("error example")},
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetHealthCheckerTTL(item.healthCheckerTTL)
		discovery.SetLoadBalancer(item.loadBalancer)

		calls := 0
		discovery.SetHealthChecker(dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
			calls++
			return item.healthCheckerError == nil, item.healthCheckerError
		}))

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		// force the health check on the first Choose run
		time.Sleep(item.healthCheckerTTL)

		for j := 0; j <= item.rerun; j++ {
			discovery.Choose()
		}

		if calls != item.expectedCalls {
			t.Errorf("scenario %d, “%s”: mismatch health check calls. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedCalls, calls)
		}

		errs := discovery.Errors()
		if !reflect.DeepEqual(errs, item.expectedErrors) {
			t.Errorf("scenario %d, “%s”: mismatch errors. Expecting: “%v”; found “%v”",
				i, item.description, item.expectedErrors, errs)
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

// ExampleDiscover is the fastest way to select a server using all default
// algorithms.
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

// ExampleDiscover_refreshAsync updates the servers list asynchronously every
// 100 milliseconds.
func ExampleDiscover_refreshAsync() {
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

// ExampleRetrieverFunc uses a specific resolver with custom timeouts.
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

// ExampleLoadBalancerFunc uses a round-robin algorithm. As we don't known which
// server position was used in the last time, we try to select using the Used
// attribute.
func ExampleLoadBalancerFunc() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetLoadBalancer(dnsdisco.LoadBalancerFunc(func(servers []dnsdisco.Server) (index int) {
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
