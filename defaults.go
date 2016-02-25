package dnsdisco

import (
	"math/rand"
	"sort"
	"time"
)

const (
	// defaultHealthCheckerTTL stores the default cache duration of the health
	// check result for a specific server.
	defaultHealthCheckerTTL = 5 * time.Second
)

var (
	randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))
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
func (d *defaultLoadBalancer) LoadBalance(servers []Server) (index int) {
	serversByPriority, priorities := groupServersByPriority(servers)

	// detect the servers that weren't selected so frequently
	minimumUsed := -1

	for _, priority := range priorities {
		selectedServers := serversByPriority[priority]

		for _, server := range selectedServers {
			if server.Used < minimumUsed || minimumUsed == -1 {
				minimumUsed = server.Used
			}
		}
	}

	// A client MUST attempt to contact the target host with the lowest-numbered
	// priority it can reach
	for _, priority := range priorities {
		selectedServers := serversByPriority[priority]

		// remove servers that are selected frequently
		for i := len(selectedServers) - 1; i >= 0; i-- {
			if selectedServers[i].Used > minimumUsed {
				selectedServers = append(selectedServers[:i], selectedServers[i+1:]...)
			}
		}

		var totalWeight int
		selectedServersWeight := make([]int, len(selectedServers))

		// compute the sum of the weights of those RRs, and with each RR
		// associate the running sum in the selected order
		for i, server := range selectedServers {
			totalWeight += int(server.Weight)
			selectedServersWeight[i] = totalWeight
		}

		// choose a uniform random number between 0 and the sum computed (inclusive)
		randomNumber := randomSource.Intn(totalWeight + 1)

		for i, weight := range selectedServersWeight {
			// select the RR whose running sum value is the first in the selected
			// order which is greater than or equal to the random number selected
			if weight < randomNumber || !selectedServers[i].LastHealthCheck {
				continue
			}

			// find the correct position of the selected server
			for j, server := range servers {
				if server == selectedServers[i] {
					return j
				}
			}
		}
	}

	return -1
}

// groupServersByPriority group the servers by priority, and also sort all
// unique priorities for a sorted access.
func groupServersByPriority(servers []Server) (map[uint16][]Server, []uint16) {
	serversByPriority := make(map[uint16][]Server)
	for _, server := range servers {
		serversByPriority[server.Priority] = append(serversByPriority[server.Priority], server)
	}

	var prioritiesTmp []int
	for priority := range serversByPriority {
		prioritiesTmp = append(prioritiesTmp, int(priority))
	}
	sort.Ints(prioritiesTmp)

	// convert back to uint16
	var priorities []uint16
	for _, priority := range prioritiesTmp {
		priorities = append(priorities, uint16(priority))
	}

	return serversByPriority, priorities
}
