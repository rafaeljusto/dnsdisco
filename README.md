dnsdisco
========

[![Build Status](https://travis-ci.org/rafaeljusto/dnsdisco.png?branch=master)](https://travis-ci.org/rafaeljusto/dnsdisco)
[![GoDoc](https://godoc.org/github.com/rafaeljusto/dnsdisco?status.png)](https://godoc.org/github.com/rafaeljusto/dnsdisco)
[![Coverage Status](https://coveralls.io/repos/github/rafaeljusto/dnsdisco/badge.svg?branch=master)](https://coveralls.io/github/rafaeljusto/dnsdisco?branch=master)

**DNS** service **disco**very library contains the following features:

* Servers are retrieved from a DNS SRV request (using your default resolver)
* Each server is verified with a health check (simple connection test)
* Load balancer choose the best server to send the request (RFC 2782 algorithm)
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

Check the [documentation](https://godoc.org/github.com/rafaeljusto/dnsdisco) for
more examples.