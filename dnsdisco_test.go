package dnsdisco

import (
	"net"
	"reflect"
	"testing"
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
