package dnsdisco

import (
	"fmt"
	"math/rand"
	"net"
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

// Discover is the fastest way to find a target using all the default
// parameters. It will send a SRV query in _service._proto.name format and
// return the first target (address and port) that passed on the health check
// (simple connection check).
//
// proto must be "udp" or "tcp", otherwise an UnknownNetworkError error will be
// returned. The library will use the local resolver to send the DNS package.
func Discover(service, proto, name string) (target string, port uint16, err error) {
	discovery := NewDiscovery(service, proto, name)
	if err = discovery.Refresh(); err != nil {
		return
	}

	target, port = discovery.Choose()
	return
}

// Discovery stores all the necessary information to discover the services,
// check if it still works and choose the best one.
type Discovery struct {
	// Service is the name of the application that the library is looking for.
	Service string

	// Proto is the protocol used by the application. Could be "udp" or "tcp".
	Proto string

	// Name is the domain name where the library will look for the SRV records.
	Name string

	// Retriever is responsible for sending the SRV requests. It is possible to
	// implement this interface to change the retrieve behaviour, that by default
	// queries the local resolver.
	Retriever retriever

	// HealthChecker is responsible for verifying if the target is still on, if
	// not the library can move to the next target. By default the health check
	// only tries a simple connection to the target.
	HealthChecker healthChecker

	// HealthCheckerTTL stores the cache time of a a health check result for a
	// specific server.
	HealthCheckerTTL time.Duration

	// Balancer is responsible for choosing the target that will be used. It has
	// the healthChecker as parameter to make it possible to choose only an online
	// target. By default the library choose the first online target from the
	// list, as it is already ordered by priority and weight.
	Balancer balancer

	// servers stores the retrieved servers to avoid DNS requests all the time.
	servers []Server
}

// NewDiscovery builds a Discovery type with all default values. To retrieve the
// servers it will use the net.LookupSRV (local resolver), for health check
// will only perform a simple connection, and the chosen target will be the
// first online one.
func NewDiscovery(service, proto, name string) Discovery {
	return Discovery{
		Service: service,
		Name:    name,
		Proto:   proto,

		Retriever: RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
			_, servers, err = net.LookupSRV(service, proto, name)
			return
		}),

		HealthChecker: HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
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
		}),
		HealthCheckerTTL: defaultHealthCheckerTTL,

		Balancer: new(defaultBalancer),
	}
}

// Refresh retrieves the servers using the DNS SRV solution. It is possible to
// change the default behaviour (local resolver with default timeouts) replacing
// the Retriever attribute from the Discovery type.
func (d *Discovery) Refresh() error {
	servers, err := d.Retriever.Retrieve(d.Service, d.Proto, d.Name)
	if err != nil {
		return err
	}

	d.servers = nil
	for _, srv := range servers {
		d.servers = append(d.servers, Server{
			SRV: *srv,
		})
	}

	return nil
}

// Choose will return the best target to use based on a defined balancer. By
// default the library choose the first online target with best priority and
// weight. It is possible to change the balancer behaviour replacing the
// Balancer attribute from the Discovery type.
func (d *Discovery) Choose() (target string, port uint16) {
	for i, server := range d.servers {
		if time.Now().Sub(server.lastHealthCheckAt) < d.HealthCheckerTTL {
			continue
		}

		ok, err := d.HealthChecker.HealthCheck(server.Target, server.Port, d.Proto)
		d.servers[i].LastHealthCheck = err == nil && ok
		d.servers[i].lastHealthCheckAt = time.Now()
	}

	// don't allow the balancer to modify the original servers slice
	serversCopy := make([]Server, len(d.servers))
	copy(serversCopy, d.servers)

	if i := d.Balancer.Balance(serversCopy); i >= 0 && i < len(d.servers) {
		d.servers[i].Used++
		return d.servers[i].Target, d.servers[i].Port
	}
	return
}

// retriever allows the library user to define a custom DNS retrieve algorithm.
type retriever interface {
	// Retrieve will send the DNS request and return all SRV records retrieved
	// from the response.
	Retrieve(service, proto, name string) ([]*net.SRV, error)
}

// RetrieverFunc is an easy-to-use implementation of the interface that is
// responsible for sending the DNS SRV requests.
type RetrieverFunc func(service, proto, name string) ([]*net.SRV, error)

// Retrieve will send the DNS request and return all SRV records retrieved from
// the response.
func (r RetrieverFunc) Retrieve(service, proto, name string) ([]*net.SRV, error) {
	return r(service, proto, name)
}

// healthChecker allows the library user to define a custom health check
// algorithm.
type healthChecker interface {
	// HealthCheck will analyze the target port/proto to check if it is still
	// capable of receiving requests.
	HealthCheck(target string, port uint16, proto string) (ok bool, err error)
}

// HealthCheckerFunc is an easy-to-use implementation of the interface that is
// responsible for checking if a target is still alive.
type HealthCheckerFunc func(target string, port uint16, proto string) (ok bool, err error)

// HealthCheck will analyze the target port/proto to check if it is still
// capable of receiving requests.
func (h HealthCheckerFunc) HealthCheck(target string, port uint16, proto string) (ok bool, err error) {
	return h(target, port, proto)
}

// balancer allows the library user to define a custom balance algorithm.
type balancer interface {
	// Balance will choose the best target.
	Balance(servers []Server) (index int)
}

// BalancerFunc is an easy-to-use implementation of the interface that is
// responsible for choosing the best target. It returns the slice index of the chosen target or -1
// when none was selected.
type BalancerFunc func(servers []Server) (index int)

// Balance will choose the best target.
func (b BalancerFunc) Balance(servers []Server) (index int) {
	return b(servers)
}

// Server stores a server information from the SRV DNS record type plus some
// extra information to control the requests for this server.
type Server struct {
	net.SRV

	// LastHealthCheck stores the result of the last health check for caching
	// purpose.
	LastHealthCheck bool

	// lastHealthCheckAt is responsible for keeping the last time that the health
	// check was performed for this server. This guarantees that we aren't going
	// to check the server every time.
	lastHealthCheckAt time.Time

	// Used stores the number of times that this server was chosen. This is useful to determinate if
	// this server will be chosen again in the future by the load balancer algorithm.
	Used int
}

// defaultBalancer is the default implementation used when the library client doesn't replace the
// Balancer attribute.
type defaultBalancer struct {
}

// Balance follows the algorithm described in the RFC 2782, based on the priority and weight of the
// SRV records.
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
func (d *defaultBalancer) Balance(servers []Server) (index int) {
	serversByPriority := make(map[uint16][]Server)
	for _, server := range servers {
		serversByPriority[server.Priority] = append(serversByPriority[server.Priority], server)
	}

	var priorities []int
	for priority := range serversByPriority {
		priorities = append(priorities, int(priority))
	}
	sort.Ints(priorities)

	var selectedTarget string

	// A client MUST attempt to contact the target host with the lowest-numbered priority it can reach
	for _, priority := range priorities {
		selectedServers := serversByPriority[uint16(priority)]

		// detect the servers that weren't selected so frequently in this priority group
		minimumUsed := -1
		for _, server := range selectedServers {
			if server.Used < minimumUsed || minimumUsed == -1 {
				minimumUsed = server.Used
			}
		}

		totalWeight := 0
		var selectedServersWeight []int

		// compute the sum of the weights of those RRs, and with each RR
		// associate the running sum in the selected order
		for i := len(selectedServers) - 1; i >= 0; i-- {
			if selectedServers[i].Used > minimumUsed {
				selectedServers = append(selectedServers[:i], selectedServers[i+1:]...)
				continue
			}

			totalWeight += int(selectedServers[i].Weight)
			selectedServersWeight = append(selectedServersWeight, totalWeight)
		}

		// choose a uniform random number between 0 and the sum computed (inclusive)
		randomNumber := randomSource.Intn(totalWeight + 1)

		for i := len(selectedServersWeight) - 1; i >= 0; i-- {
			// select the RR whose running sum value is the first in the selected order which is greater
			// than or equal to the random number selected
			if selectedServersWeight[i] >= randomNumber && selectedServers[i].LastHealthCheck {
				selectedTarget = selectedServers[i].Target
				break
			}
		}

		if selectedTarget != "" {
			break
		}
	}

	// find the correct position of the selected server
	for i, server := range servers {
		if server.Target == selectedTarget {
			return i
		}
	}

	return -1
}
