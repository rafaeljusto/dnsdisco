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

var discoverScenarios = []struct {
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

var refreshAsyncScenarios = []struct {
	description     string
	service         string
	proto           string
	name            string
	expectedTarget  string
	expectedPort    uint16
	expectedError   error
	refreshInterval time.Duration
	retriever       dnsdisco.RetrieverFunc
	healthChecker   dnsdisco.HealthCheckerFunc
	expectedErrors  []error
}{
	{
		description:     "it should update the servers asynchronously",
		service:         "jabber",
		proto:           "tcp",
		name:            "registro.br",
		expectedTarget:  "server4.example.com.",
		expectedPort:    4444,
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
	},
	{
		description:     "it should fail to retrieve the SRV records",
		service:         "jabber",
		proto:           "tcp",
		name:            "registro.br",
		expectedTarget:  "server1.example.com.",
		expectedPort:    1111,
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
		expectedErrors: []error{
			net.UnknownNetworkError("test"),
		},
	},
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	for i, item := range discoverScenarios {
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

func TestRefreshAsync(t *testing.T) {
	t.Parallel()

	for i, item := range refreshAsyncScenarios {
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
