package dnsdisco

import "time"

const (
	// defaultHealthCheckerTTL stores the default cache duration of the health
	// check result for a specific server.
	defaultHealthCheckerTTL = 5 * time.Second
)

// defaultLoadBalancer is the default implementation used when the library
// client doesn't replace using the SetLoadBalancer method.
type defaultLoadBalancer struct {
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
func (d defaultLoadBalancer) LoadBalance(servers []Server) (index int) {
	var selectedServers []serverWeight
	var totalWeight int

	priority := -1
	minimumUse := d.getServersMinimumUse(servers)

	for i, server := range servers {
		// detect priority change
		if priority != -1 && priority != int(server.Priority) {
			break
		}

		if server.Used == minimumUse && server.LastHealthCheck {
			priority = int(server.Priority)
			totalWeight += int(server.Weight)
			selectedServers = append(selectedServers, serverWeight{
				Server:        server,
				weight:        totalWeight,
				originalIndex: i,
			})
		}
	}

	// choose a uniform random number between 0 and the sum computed (inclusive)
	randomNumber := randomSource.Intn(totalWeight + 1)

	for _, server := range selectedServers {
		// select the RR whose running sum value is the first in the selected
		// order which is greater than or equal to the random number selected
		if server.weight >= randomNumber {
			return server.originalIndex
		}
	}

	return -1
}

// getServersMinimumUse returns the minimum number of times that a server was
// selected. If no server is available -1 is returned.
func (d defaultLoadBalancer) getServersMinimumUse(servers []Server) int {
	minimumUsed := -1
	for _, server := range servers {
		if (server.Used < minimumUsed || minimumUsed == -1) && server.LastHealthCheck {
			minimumUsed = server.Used
		}
	}
	return minimumUsed
}

// serverWeight stores a server type plus some additional data useful for
// selecting the server according the RFC 2782 algorithm.
type serverWeight struct {
	Server

	// weight compute the sum of the weights of the running sum in the selected
	// order.
	weight int

	// originalIndex stores the index reference from the original slice of
	// servers.
	originalIndex int
}
