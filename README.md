# Golang-Discovery

This is a work in progess.

## How it works

Golang-Discovery uses multicast to advertise what services a local machine has exposed. Those services can then be automatically consumes through the use of a an object that implements the `Generator` interface.

## Example

Lets say that you want to automatically keep a list of all services of a type that are alive. This is how you would do it.

```
import (
  "github.com/nanbox-io/golang-discovery"
  "time"
  "io"
  "fmt"
)

type (
  generator struct {}
)

func main() {
  discover, err := discovery.NewDiscovery("eth0", "secret", time.Second)
  if err != nil {
    panic(err)
  }

  discover.Add("service name", "value")
  discover.Handle("service name", generator{})

  err := discover.Loop()
  if err != nil {
    panic(err)
  }
}


func (g generator) New(address) io.Closer {
  fmt.Println("creating a new client for", address)
  return io.NoopCloser(nil)
}

```