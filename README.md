dnsdisco
========

[![Build Status](https://travis-ci.org/rafaeljusto/dnsdisco.png?branch=master)](https://travis-ci.org/rafaeljusto/dnsdisco)
[![GoDoc](https://godoc.org/github.com/rafaeljusto/dnsdisco?status.png)](https://godoc.org/github.com/rafaeljusto/dnsdisco)
[![Coverage Status](https://coveralls.io/repos/github/rafaeljusto/dnsdisco/badge.svg?branch=master)](https://coveralls.io/github/rafaeljusto/dnsdisco?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/rafaeljusto/dnsdisco)](https://goreportcard.com/report/github.com/rafaeljusto/dnsdisco)

**DNS** service **disco**very library.


Motivation
----------

So you have more than one service (or microservice) and you need to integrate
them? Great! I can think on some options to store/retrieve the services address
to allow this integration:

* Configuration file
* Centralized configuration system (etcd, etc.)
* Load balancer device ($$$) or software
* DNS

Wait? What? DNS? Yep! You can use the [SRV
records](https://tools.ietf.org/html/rfc2782) to announce your service
addresses. And with a small TTL (cache) you could also make fast transitions to
increase/decrease the number of instances.

This SRV records contains priority and weight, so you can determinate which
servers (of the same name) receives more requests than others. The only problem
was that with the DNS solution we didn't have the health check feature of a load
balancer. And that's where this library jumps in! It will choose for you the
best server looking the priority, weight and the health check result. And as
each service has it's particular way of health checking, this library is
flexible so you can implement the best health check algorithm that fits for you.

And doesn't stop there. If you don't want to use the default resolver for
retrieving the SRV records, you can change it! If you want a more efficient load
balancer algorithm (please send a PR :smile:), you can also implement it.

This library follows the Go language philosophy:
> "Less is more" (Ludwig Mies van der Rohe)


Features
--------

* Servers are retrieved from a DNS SRV request (using your default resolver)
* Each server is verified with a health check (simple connection test)
* Load balancer choose the best server to send the request (RFC 2782 algorithm)
* Library is flexible so you could change any part with your own implementation
* Has only standard library dependencies (except for tests)


Example
-------

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

For this example we imagine that the domain **registro.br.** is configured as
the following:

```dns
registro.br.    172800 IN SOA a.dns.br. hostmaster.registro.br. (
                        2016021101 ; serial
                        86400      ; refresh (1 day)
                        3600       ; retry (1 hour)
                        604800     ; expire (1 week)
                        86400      ; minimum (1 day)
                        )

registro.br.    172800 IN NS a.dns.br.
registro.br.    172800 IN NS b.dns.br.
registro.br.    172800 IN NS c.dns.br.
registro.br.    172800 IN NS d.dns.br.
registro.br.    172800 IN NS e.dns.br.

_jabber._tcp.registro.br. 172800 IN SRV	1 65534 5269 jabber.registro.br.
```

Check the [documentation](https://godoc.org/github.com/rafaeljusto/dnsdisco) for
more examples.