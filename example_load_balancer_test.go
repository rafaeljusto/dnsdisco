package dnsdisco_test

import (
	"container/ring"
	"fmt"
	"net"

	"github.com/rafaeljusto/dnsdisco"
)

// roundRobinLoadBalancer is a load balancer that selects the server using a
// round robin algorithm.
type roundRobinLoadBalancer struct {
	// servers is a circular linked list to allow a fast round robin algorithm.
	servers *ring.Ring
}

// ChangeServers will be called anytime that a new set of servers is retrieved.
func (r *roundRobinLoadBalancer) ChangeServers(servers []*net.SRV) {
	r.servers = ring.New(len(servers))
	i, n := 0, r.servers.Len()

	for p := r.servers; i < n; p = p.Next() {
		p.Value = servers[i]
		i++
	}
}

// LoadBalance will choose the best target based on a round robin strategy. If
// no server is selected an empty target and a zero port is returned.
func (d roundRobinLoadBalancer) LoadBalance() (target string, port uint16) {
	if d.servers.Len() == 0 {
		return "", 0
	}

	server, _ := d.servers.Value.(*net.SRV)
	d.servers = d.servers.Next()
	return server.Target, server.Port
}

// Example shows how it is possible to replace the default load balancer
// algorithm with a new one following the round robin strategy
// (https://en.wikipedia.org/wiki/Round-robin_scheduling).
func Example() {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetLoadBalancer(new(roundRobinLoadBalancer))

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
