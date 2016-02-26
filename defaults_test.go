package dnsdisco_test

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/rafaeljusto/dnsdisco"
)

func TestDefaultLoadBalancer(t *testing.T) {
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
			description: "it should fallback inside priority group",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server2.example.com.", nil
			}),
			expectedTarget: "server2.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should fallback to other priority group by health check",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
					{
						Target:   "server3.example.com.",
						Port:     3333,
						Priority: 20,
						Weight:   20,
					},
					{
						Target:   "server4.example.com.",
						Port:     4444,
						Priority: 20,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server4.example.com.", nil
			}),
			expectedTarget: "server4.example.com.",
			expectedPort:   4444,
		},
		{
			description: "it should fallback to other priority group by used counter",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   10,
					},
					{
						Target:   "server3.example.com.",
						Port:     3333,
						Priority: 20,
						Weight:   20,
					},
					{
						Target:   "server4.example.com.",
						Port:     4444,
						Priority: 20,
						Weight:   10,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target != "server3.example.com.", nil
			}),
			rerun:          2,
			expectedTarget: "server4.example.com.",
			expectedPort:   4444,
		},
		{
			description: "it should select the less used server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					{
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
			description: "it should retrieve the target correctly (same target different port)",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   0,
					},
					{
						Target:   "server1.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   200,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server1.example.com.", nil
			}),
			expectedTarget: "server1.example.com.",
			expectedPort:   2222,
		},
		{
			description: "it should not select any target",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 10,
						Weight:   200,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 10,
						Weight:   0,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return false, nil
			}),
			expectedTarget: "",
			expectedPort:   0,
		},
		{
			description: "it should ignore a sick server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "server1.example.com.",
						Port:     1111,
						Priority: 1,
						Weight:   20,
					},
					{
						Target:   "server2.example.com.",
						Port:     2222,
						Priority: 2,
						Weight:   20,
					},
				}, nil
			}),
			healthChecker: dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
				return target == "server1.example.com.", nil
			}),
			rerun:          1,
			expectedTarget: "server1.example.com.",
			expectedPort:   1111,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetHealthChecker(item.healthChecker)

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		var target string
		var port uint16

		for j := 0; j <= item.rerun; j++ {
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

func TestDefaultHealthChecker(t *testing.T) {
	ln, err := startTCPTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	testServerHost, p, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	testServerPort, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		description    string
		service        string
		proto          string
		name           string
		retriever      dnsdisco.RetrieverFunc
		loadBalancer   loadBalacerMock
		expectedTarget string
		expectedPort   uint16
		expectedError  error
	}{
		{
			description: "it should identify a healthy server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   testServerHost,
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			loadBalancer: func() loadBalacerMock {
				var savedServers []*net.SRV
				return loadBalacerMock{
					MockChangeServers: func(servers []*net.SRV) {
						savedServers = servers
					},
					MockLoadBalance: func() (target string, port uint16) {
						if len(savedServers) > 0 {
							return savedServers[0].Target, savedServers[0].Port
						}

						return "", 0
					},
				}
			}(),
			expectedTarget: testServerHost,
			expectedPort:   uint16(testServerPort),
		},
		{
			description: "it should fail when it's not a valid proto",
			service:     "jabber",
			proto:       "xxx",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   testServerHost,
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			loadBalancer: func() loadBalacerMock {
				var savedServers []*net.SRV
				return loadBalacerMock{
					MockChangeServers: func(servers []*net.SRV) {
						savedServers = servers
					},
					MockLoadBalance: func() (target string, port uint16) {
						if len(savedServers) > 0 {
							return savedServers[0].Target, savedServers[0].Port
						}

						return "", 0
					},
				}
			}(),
			expectedTarget: "",
			expectedPort:   0,
		},
		{
			description: "it should fail to connect to an unknown server",
			service:     "jabber",
			proto:       "tcp",
			name:        "registro.br",
			retriever: dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
				return []*net.SRV{
					{
						Target:   "idontexist.example.com.",
						Port:     uint16(testServerPort),
						Priority: 10,
						Weight:   20,
					},
				}, nil
			}),
			loadBalancer: func() loadBalacerMock {
				var savedServers []*net.SRV
				return loadBalacerMock{
					MockChangeServers: func(servers []*net.SRV) {
						savedServers = servers
					},
					MockLoadBalance: func() (target string, port uint16) {
						if len(savedServers) > 0 {
							return savedServers[0].Target, savedServers[0].Port
						}

						return "", 0
					},
				}
			}(),
			expectedTarget: "",
			expectedPort:   0,
		},
	}

	for i, item := range scenarios {
		discovery := dnsdisco.NewDiscovery(item.service, item.proto, item.name)
		discovery.SetRetriever(item.retriever)
		discovery.SetLoadBalancer(item.loadBalancer)

		if err := discovery.Refresh(); err != nil {
			t.Errorf("scenario %d, “%s”: unexpected error while retrieving DNS records. Details: %s",
				i, item.description, err)
		}

		target, port := discovery.Choose()

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

func BenchmarkDefaultLoadBalancer(b *testing.B) {
	discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
	discovery.SetHealthChecker(dnsdisco.HealthCheckerFunc(func(target string, port uint16, proto string) (ok bool, err error) {
		return true, nil
	}))

	discovery.SetRetriever(dnsdisco.RetrieverFunc(func(service, proto, name string) ([]*net.SRV, error) {
		return []*net.SRV{
			{
				Target:   "server1.example.com.",
				Port:     1111,
				Weight:   10,
				Priority: 20,
			},
			{
				Target:   "server2.example.com.",
				Port:     2222,
				Weight:   70,
				Priority: 10,
			},
			{
				Target:   "server3.example.com.",
				Port:     3333,
				Weight:   100,
				Priority: 20,
			},
			{
				Target:   "server4.example.com.",
				Port:     4444,
				Weight:   1,
				Priority: 15,
			},
			{
				Target:   "server5.example.com.",
				Port:     5555,
				Weight:   40,
				Priority: 60,
			},
		}, nil
	}))

	// Retrieve the servers
	if err := discovery.Refresh(); err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < b.N; i++ {
		discovery.Choose()
	}
}

// startTCPTestServer initialize a TCP echo server running on any available port
// of the localhost. The returning listener must be closed to terminate the
// server.
func startTCPTestServer() (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				break
			}
			// Server connection.
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)

				n, err := c.Read(buf)
				if err != nil {
					return
				}
				c.Write(buf[:n])
			}(c)
		}
	}()

	return ln, nil
}

// loadBalacerMock creates an easy way to test a load balancer.
type loadBalacerMock struct {
	// MockChangeServers will be called anytime that a new set of servers is
	// retrieved.
	MockChangeServers func(servers []*net.SRV)

	// MockLoadBalance will choose the best target.
	MockLoadBalance func() (target string, port uint16)
}

// ChangeServers will be called anytime that a new set of servers is retrieved.
func (l loadBalacerMock) ChangeServers(servers []*net.SRV) {
	l.MockChangeServers(servers)
}

// LoadBalance will choose the best target.
func (l loadBalacerMock) LoadBalance() (target string, port uint16) {
	return l.MockLoadBalance()
}
