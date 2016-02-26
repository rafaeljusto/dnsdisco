// +build dev

package dnsdisco

import (
	"net"
	"strconv"
)

var (
	// DevTarget stores the target that will be used in the test environment. This
	// should be replaced with ldflags for what you're really going to use.
	DevTarget string = "localhost"

	// DevTarget stores the port that will be used in the test environment. This
	// should be replaced with ldflags for what you're really going to use. If you
	// inform an invalid port number (e.g "XXX") the Retriever will return an
	// error.
	DevPort string = "80"
)

// To make it easy in test environments to test the system without configuring a
// DNS server, you can compile your project with the following flags:
//
//  go build -tags "dev" -ldflags "-X github.com/rafaeljusto/dnsdisco.DevTarget=localhost -X github.com/rafaeljusto/dnsdisco.DevPort=443"
//
// Where you should replace:
//   * "localhost" for your server address in the test environment
//   * "443" for your server port in the test environment
func init() {
	DefaultRetriever = RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
		port, err := strconv.ParseUint(DevPort, 10, 16)
		if err != nil {
			return nil, err
		}

		return []*net.SRV{
			&net.SRV{
				Target: DevTarget,
				Port:   uint16(port),
			},
		}, nil
	})
}
