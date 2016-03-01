package dnsdisco

import (
	"fmt"
	"net"
)

// NewDefaultRetriever returns an instance of the default retriever algorithm,
// that uses the local resolver to retrieve the SRV records.
func NewDefaultRetriever() Retriever {
	return RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
		_, servers, err = net.LookupSRV(service, proto, name)
		return
	})
}

// NewDefaultHealthChecker returns an instance of the default health checker
// algorithm. The default health checker tries to do a simple connection to the
// server. If the connection is successful the health check pass, otherwise it
// fails with an error. Possible proto values are tcp or udp.
func NewDefaultHealthChecker() HealthChecker {
	return HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
		address := fmt.Sprintf("%s:%d", target, port)
		if proto != "tcp" && proto != "udp" {
			return false, net.UnknownNetworkError(proto)
		}

		conn, err := net.Dial(proto, address)
		if err != nil {
			return false, err
		}
		conn.Close()
		return true, nil
	})
}

// NewDefaultLoadBalancer returns an instance of the default load balancer
// algorithm, that selects the best server based on the RFC 2782 algorithm.
// If no server is selected an empty target and a zero port is returned.
func NewDefaultLoadBalancer() LoadBalancer {
	return new(defaultLoadBalancer)
}

// defaultLoadBalancer is the default implementation used when the library
// client doesn't replace using the SetLoadBalancer method.
type defaultLoadBalancer struct {
	servers []defaultLoadBalancerServer
}

// ChangeServers will be called anytime that a new set of servers is retrieved.
// The library grantees that this is go routine safe.
func (d *defaultLoadBalancer) ChangeServers(servers []*net.SRV) {
	d.servers = nil
	for _, server := range servers {
		d.servers = append(d.servers, defaultLoadBalancerServer{
			SRV: *server,
		})
	}
}

// LoadBalance follows the algorithm described in the RFC 2782, based on the
// priority and weight of the SRV records.
//
//   Compute the sum of the weights of those RRs, and with each RR
//   associate the running sum in the selected order. Then choose a
//   uniform random number between 0 and the sum computed
//   (inclusive), and select the RR whose running sum value is the
//   first in the selected order which is greater than or equal to
//   the random number selected. The target host specified in the
//   selected SRV RR is the next one to be contacted by the client.
//   Remove this SRV RR from the set of the unordered SRV RRs and
//   apply the described algorithm to the unordered SRV RRs to select
//   the next target host.  Continue the ordering process until there
//   are no unordered SRV RRs.  This process is repeated for each
//   Priority.
//
// The algorithm assumes that the servers slice is already sorted by priority
// and randomized by weight within a priority.
func (d defaultLoadBalancer) LoadBalance() (target string, port uint16) {
	var selectedServers []defaultLoadBalancerServer
	var totalWeight int

	priority := -1
	minimumUse := d.getServersMinimumUse()

	for i, server := range d.servers {
		// detect priority change
		if priority != -1 && priority != int(server.Priority) {
			break
		}

		if server.selected == minimumUse {
			priority = int(server.Priority)
			totalWeight += int(server.Weight)

			server.weightSum = totalWeight
			server.originalIndex = i
			selectedServers = append(selectedServers, server)
		}
	}

	// choose a uniform random number between 0 and the sum computed (inclusive)
	randomNumber := randomSource.Intn(totalWeight + 1)

	for _, server := range selectedServers {
		// select the RR whose running sum value is the first in the selected
		// order which is greater than or equal to the random number selected
		if server.weightSum >= randomNumber {
			d.servers[server.originalIndex].selected++
			return server.Target, server.Port
		}
	}

	return "", 0
}

// getServersMinimumUse returns the minimum number of times that a server was
// selected. If no server is available -1 is returned.
func (d defaultLoadBalancer) getServersMinimumUse() int {
	minimumUsed := -1
	for _, server := range d.servers {
		if server.selected < minimumUsed || minimumUsed == -1 {
			minimumUsed = server.selected
		}
	}
	return minimumUsed
}

// defaultLoadBalancerServer stores a server type plus some additional data
// useful for selecting the server according the RFC 2782 algorithm.
type defaultLoadBalancerServer struct {
	net.SRV

	// weightSum compute the sum of the weights of the running sum in the selected
	// order.
	weightSum int

	// selected is the number of times that a server was selected by the load
	// balancer algorithm.
	selected int

	// originalIndex stores the index reference from the original slice of
	// servers.
	originalIndex int
}
