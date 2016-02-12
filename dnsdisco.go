// Package dnsdisco is a DNS service discovery library with health check and
// load balancer features.
//
// The library is very flexible and uses interfaces everywhere to make it
// possible for the library user to replace any part with a custom algorithm. A
// basic use would be:
//
//    package main
//
//    import (
//      "fmt"
//      "github.com/rafaeljusto/dnsdisco"
//    )
//
//    func main() {
//      target, port, err := dnsdisco.Discover("jabber", "tcp", "registro.br")
//      if err != nil {
//        fmt.Println(err)
//        return
//      }
//
//      fmt.Printf("Target: %s\nPort: %d\n", target, port)
//    }
package dnsdisco

import (
	"fmt"
	"net"
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

	// Balancer is responsible for choosing the target that will be used. It has
	// the healthChecker as parameter to make it possible to choose only an online
	// target. By default the library choose the first online target from the
	// list, as it is already ordered by priority and weight.
	Balancer balancer

	// services stores the retrieved services to avoid DNS requests all the time.
	services []*net.SRV
}

// NewDiscovery builds a Discovery type with all default values. To retrieve the
// services it will use the net.LookupSRV (local resolver), for health check
// will only perform a simple connection, and the chosen target will be the
// first online one.
func NewDiscovery(service, proto, name string) Discovery {
	return Discovery{
		Service: service,
		Name:    name,
		Proto:   proto,

		Retriever: RetrieverFunc(func(service, proto, name string) (services []*net.SRV, err error) {
			_, services, err = net.LookupSRV(service, proto, name)
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

		Balancer: BalancerFunc(func(services []*net.SRV, healthCheck healthChecker, proto string) (index int) {
			for i, service := range services {
				ok, err := healthCheck.HealthCheck(service.Target, service.Port, proto)
				if err != nil || !ok {
					continue
				}

				return i
			}

			return -1
		}),
	}
}

// Refresh retrieves the services using the DNS SRV solution. It is possible to
// change the default behaviour (local resolver with default timeouts) replacing
// the Retriever attribute from the Discovery type.
func (d *Discovery) Refresh() (err error) {
	d.services, err = d.Retriever.Retrieve(d.Service, d.Proto, d.Name)
	return
}

// Choose will return the best target to use based on a defined balancer. By
// default the library choose the first online target with best priority and
// weight. It is possible to change the balancer behaviour replacing the
// Balancer attribute from the Discovery type.
func (d Discovery) Choose() (target string, port uint16) {
	if i := d.Balancer.Balance(d.services, d.HealthChecker, d.Proto); i >= 0 && i < len(d.services) {
		return d.services[i].Target, d.services[i].Port
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
	Balance(services []*net.SRV, healthCheck healthChecker, proto string) (index int)
}

// BalancerFunc is an easy-to-use implementation of the interface that is
// responsible for choosing the best target.
type BalancerFunc func(services []*net.SRV, healthCheck healthChecker, proto string) (index int)

// Balance will choose the best target.
func (b BalancerFunc) Balance(services []*net.SRV, healthCheck healthChecker, proto string) (index int) {
	return b(services, healthCheck, proto)
}
