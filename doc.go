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
//
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
//
// Now you want to use your own resolver with specific timeouts to retrieve the
// servers:
//
//    package main
//
//    import (
//      "fmt"
//      "net"
//      "strings"
//      "time"
//
//      "github.com/miekg/dns"
//      "github.com/rafaeljusto/dnsdisco"
//    )
//
//    func main() {
//      discovery := dnsdisco.NewDiscovery("jabber", "tcp", "registro.br")
//      discovery.Retriever = dnsdisco.RetrieverFunc(func(service, proto, name string) (servers []*net.SRV, err error) {
//        client := dns.Client{
//          ReadTimeout: 2*time.Second,
//          WriteTimeout: 2*time.Second,
//        }
//
//        name = strings.TrimRight(name, ".")
//        z := fmt.Sprintf("_%s._%s.%s.", service, proto, name)
//
//        var request dns.Msg
//        request.SetQuestion(z, dns.TypeSRV)
//        request.RecursionDesired = true
//
//        response, _, err := client.Exchange(&request, "8.8.8.8:53")
//        if err != nil {
//          return nil, err
//        }
//
//        for _, rr := range response.Answer {
//          if srv, ok := rr.(*dns.SRV); ok {
//            servers = append(servers, &net.SRV{
//              Target: srv.Target,
//              Port: srv.Port,
//              Priority: srv.Priority,
//              Weight: srv.Weight,
//            })
//          }
//        }
//
//        return
//      })
//
//      // Retrieve the servers
//      if err := discovery.Refresh(); err != nil {
//        fmt.Println(err)
//        return
//      }
//
//      target, port := discovery.Choose()
//      fmt.Printf("Target: %s\nPort: %d\n", target, port)
//    }
package dnsdisco
