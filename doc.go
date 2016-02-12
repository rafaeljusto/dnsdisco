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
