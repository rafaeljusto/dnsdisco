package dnsdisco_test

import (
	"fmt"
	"net"
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
					&net.SRV{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					&net.SRV{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				switch target {
				case "server1.example.com.":
					return false, nil
				case "server2.example.com.":
					return true, nil
				}

				return false, nil
			}),
			expectedTarget: "server2.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should retrieve the target correctly (fallback to other priority group)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					&net.SRV{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					&net.SRV{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
					&net.SRV{
						Target:   "server3.example.com.",
						Port:     3333,
						Priority: 20,
						Weight:   20,
					},
					&net.SRV{
						Target:   "server4.example.com.",
						Port:     4444,
						Priority: 20,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				switch target {
				case "server1.example.com.":
					return false, nil
				case "server2.example.com.":
					return false, nil
				case "server3.example.com.":
					return false, nil
				case "server4.example.com.":
					return true, nil
				}

				return false, nil
			}),
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
					&net.SRV{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					&net.SRV{
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
			description: "it should not select any target",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					&net.SRV{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					&net.SRV{
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
					return false, nil
				case "server2.example.com.":
					return false, nil
				}

				return false, nil
			}),
			expectedTarget: "",
			expectedPort:   0,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.Retriever = item.retriever
		discovery.HealthChecker = item.healthChecker

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, item.expectedTarget, err)
		}

		var target string
		var port uint16

		for i := 0; i <= item.rerun; i++ {
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

func ExampleRetrieverFunc() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.Retriever = dnsdisco.RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
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
	})

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
