dnsdisco
========

[![GoDoc](https://godoc.org/github.com/rafaeljusto/dnsdisco?status.png)](https://godoc.org/github.com/rafaeljusto/dnsdisco)

**DNS** service **disco**very library contains the following features:

* Servers are retrieved from a DNS SRV request (using your default resolver)
* Each server is verified with a health check (simple connection test)
* Load balancer choose the best server to send the request (first high priority online server is used)
* Library is flexible so you could change any part with your own implementation

A basic use would be:

```go
package main

import (
  "fmt"

  "github.com/rafaeljusto/dnsdisco"
)

func main() {
  target, port, err := dnsdisco.Discover("jabber", "tcp", "registro.br")
  if err != nil {
    fmt.Println(err)
    return
  }

  fmt.Printf("Target: %s\nPort: %d\n", target, port)
}
```

Now you want to use your own resolver with specific timeouts to retrieve the servers:

```go
package main

import (
  "fmt"
  "net"
  "strings"
  "time"

  "github.com/miekg/dns"
  "github.com/rafaeljusto/dnsdisco"
)

func main() {
  discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
  discovery.Retriever = dnsdisco.RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
    client := dns.Client{
      ReadTimeout: 2*time.Second,
      WriteTimeout: 2*time.Second,
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
          Target: srv.Target,
          Port: srv.Port,
          Priority: srv.Priority,
          Weight: srv.Weight,
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
}
```
