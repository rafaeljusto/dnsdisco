package dnsdisco

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
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
				Err:    "no such host",
				Name:   "_jabber._xxx.registro.br",
				Server: "200.160.3.2:53",
			},
		},
	}

	for i, item := range scenarios {
		target, port, err := Discover(item.service, item.proto, item.name)

		if target != item.expectedTarget {
			t.Errorf("scenario %d, “%s”: mismatch targets. Expecting: “%s”; found “%s”",
				i, item.description, item.expectedTarget, target)
		}

		if port != item.expectedPort {
			t.Errorf("scenario %d, “%s”: mismatch ports. Expecting: “%d”; found “%d”",
				i, item.description, item.expectedPort, port)
		}

		if !reflect.DeepEqual(err, item.expectedError) {
			t.Errorf("scenario %d, “%s”: mismatch errors. Expecting: “%v”; found “%v”",
				i, item.description, item.expectedError, err)
		}
	}
}

func ExampleDiscover() {
	target, port, err := Discover("jabber", "tcp", "registro.br")
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
	discovery := NewDiscovery("jabber", "tcp", "registro.br")
	discovery.Retriever = RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
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
